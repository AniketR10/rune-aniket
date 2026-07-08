// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package symboldb

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	bluebolt "github.com/unstablebuild/blue/document/bolt"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/idelsp/symbolresolve"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/workspace/walkdir"
)

// EditorEvents returns the editor event types the Parser must be
// subscribed to in order to keep the index fresh: in-editor saves and
// the out-of-band disk events. Buffer edits are deliberately excluded
// because they do not touch disk.
func EditorEvents() []textapi.EventType {
	return []textapi.EventType{
		textapi.EventTypeFlush,
		textapi.EventTypeChange,
		textapi.EventTypeCreate,
		textapi.EventTypeRemove,
		textapi.EventTypeRename,
	}
}

// Backing is the parser the index wraps: the full editor parser surface
// for query delegation plus a factory for the batched query sessions
// the indexer runs extraction through. syntax.Parser satisfies it.
type Backing interface {
	syntaxapi.Parser
	// NewQuerySession returns a per-goroutine batched file querier.
	NewQuerySession() symbolresolve.QuerySession
}

// Parser is an index-backed syntaxapi.Parser. It also implements
// text.EventHandler to invalidate per-file index entries and io.Closer
// to stop the indexer goroutine. The storage service is caller-owned
// and is not closed by Close.
type Parser struct {
	backing Backing
	fs      workspaceapi.FileSystem
	root    workspaceapi.URI
	notify  browserapi.Notifications
	// schedule enqueues a closure onto the editor event loop. All
	// Notifications calls go through it: the production implementation
	// reads focus state and mutates UI components the loop owns.
	schedule func(func()) bool

	// files, symbols and names are separate partitions because List is
	// the storage API's only enumeration primitive and it decodes a
	// whole partition: the scan's stat-only restart path lists file
	// records without deserializing the (much larger) symbols table,
	// and ListReferencedSymbols streams the tiny name markers without
	// decoding any symbol locations. meta is the caller's db itself,
	// holding only the scan marker at its root.
	files   storageapi.Service
	symbols storageapi.Service
	names   storageapi.Service
	meta    storageapi.Service

	ctx    context.Context
	cancel context.CancelFunc

	mu sync.Mutex
	// dirty is the coalescing set of file URIs pending re-index.
	dirty map[string]struct{}
	// busy is true while the worker processes a dirty entry.
	busy bool
	// scanned is true once this session's full scan completed.
	scanned bool
	// scannedEver is true once any scan has ever completed over this
	// database (persisted marker); it gates ListReferencedSymbols.
	scannedEver bool
	closed      bool
	// updated is closed (and replaced) on every indexer state change so
	// Wait can block without polling.
	updated chan struct{}

	// wake nudges the worker when the dirty set becomes non-empty.
	wake chan struct{}
	// done is closed when the worker goroutine exits.
	done chan struct{}
}

var _ syntaxapi.Parser = (*Parser)(nil)

// New returns an index-backed Parser over the given backing parser and
// workspace filesystem, persisting its database in db. Indexing starts
// immediately in the background; queries are served from whatever
// state the database is in. Indexing progress is reported through
// notify, always from closures run by schedule on the editor event
// loop.
func New(
	backing Backing, fs workspaceapi.FileSystem,
	root workspaceapi.URI, db storageapi.Service,
	notify browserapi.Notifications,
	schedule func(func()) bool,
) (*Parser, error) {
	files, err := db.Partition(filesPartition)
	if err != nil {
		return nil, err
	}
	symbols, err := db.Partition(symbolsPartition)
	if err != nil {
		return nil, err
	}
	names, err := db.Partition(namesPartition)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Parser{
		backing:  backing,
		fs:       fs,
		root:     root,
		notify:   notify,
		schedule: schedule,
		files:    files,
		symbols:  symbols,
		names:    names,
		meta:     db,
		ctx:      ctx,
		cancel:   cancel,
		dirty:    make(map[string]struct{}),
		updated:  make(chan struct{}),
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	// A marker written by another schema version must not enable
	// index-served listing: the derived tables it vouches for may not
	// exist under that older layout.
	var m metaDoc
	if err := db.Get(ctx, metaScanID, &m); err == nil && m.Version == schemaVersion {
		p.scannedEver = m.Complete
	}
	go debug.CapturePanicReport(p.run)
	return p, nil
}

// Handle implements text.EventHandler. It is O(1) and never blocks:
// relevant events only mark the file dirty for the indexer goroutine.
func (p *Parser) Handle(_ context.Context, ev textapi.Event) bool {
	switch ev.Type {
	case textapi.EventTypeFlush, textapi.EventTypeChange,
		textapi.EventTypeCreate, textapi.EventTypeRemove,
		textapi.EventTypeRename:
	default:
		return false
	}
	us := ev.URI.String()
	if symbolresolve.SpecForFile(us) == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return true
	}
	p.dirty[us] = struct{}{}
	p.wakeLocked()
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return false
}

// Wait blocks until the initial scan has completed and the dirty queue
// has drained. It is intended for tests and diagnostics.
func (p *Parser) Wait(ctx context.Context) error {
	for {
		p.mu.Lock()
		if p.scanned && !p.busy && len(p.dirty) == 0 {
			p.mu.Unlock()
			return nil
		}
		updated := p.updated
		p.mu.Unlock()
		select {
		case <-updated:
		case <-ctx.Done():
			return ctx.Err()
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

// Close stops the indexer goroutine. The storage service is
// caller-owned and left open.
func (p *Parser) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.cancel()
	<-p.done
	return nil
}

// wakeLocked wakes every Wait call blocked on a state change.
// Callers must hold p.mu.
func (p *Parser) wakeLocked() {
	close(p.updated)
	p.updated = make(chan struct{})
}

// setScanned records that this session's full scan finished.
func (p *Parser) setScanned() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scanned = true
	if p.ctx.Err() == nil {
		p.scannedEver = true
	}
	p.wakeLocked()
}

// nextDirty pops one dirty entry, tracking the worker's busy state so
// Wait cannot observe an empty queue while a job is still in flight.
func (p *Parser) nextDirty() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for us := range p.dirty {
		delete(p.dirty, us)
		p.busy = true
		return us, true
	}
	if p.busy {
		p.busy = false
		p.wakeLocked()
	}
	return "", false
}

func (p *Parser) hasScannedEver() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scannedEver
}

// Search delegates to the backing parser.
func (p *Parser) Search(
	query string, captureNames []string, languages ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return p.backing.Search(query, captureNames, languages...)
}

// SearchNode delegates to the backing parser.
func (p *Parser) SearchNode(
	nodeTypes syntaxapi.NodeCaptureName, languages ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return p.backing.SearchNode(nodeTypes, languages...)
}

// Query delegates to the backing parser.
func (p *Parser) Query(
	file workspaceapi.URI, query string, captureNames []string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return p.backing.Query(file, query, captureNames)
}

// QueryNode delegates to the backing parser.
func (p *Parser) QueryNode(
	file workspaceapi.URI, nodeTypes syntaxapi.NodeCaptureName,
) (iterator.Iterator[syntaxapi.Result], error) {
	return p.backing.QueryNode(file, nodeTypes)
}

// Highlight delegates to the backing parser.
func (p *Parser) Highlight(
	uri workspaceapi.URI, content string,
) (iterator.Iterator[textapi.Location], error) {
	return p.backing.Highlight(uri, content)
}

// ResolveSymbol serves a symbol lookup from the index, passing through
// to the backing parser only when the symbol has no database entry.
// Indexed results may be stale while a rebuild is in flight.
func (p *Parser) ResolveSymbol(
	ctx context.Context, name string, progress syntaxapi.Progress,
) (iterator.Iterator[syntaxapi.Match], error) {
	if !strings.Contains(name, ".") {
		return nil, syntaxapi.ErrNoDot
	}
	matches, ok := p.resolveFromIndex(ctx, name, progress)
	if !ok {
		return p.backing.ResolveSymbol(ctx, name, progress)
	}
	return iterator.FromSlice(matches), nil
}

// resolveFromIndex builds matches for name from its symbol doc. It
// reports ok=false on any miss so the caller can pass through.
func (p *Parser) resolveFromIndex(
	ctx context.Context, name string, progress syntaxapi.Progress,
) ([]syntaxapi.Match, bool) {
	parts := strings.Split(name, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, false
	}
	var doc symbolDoc
	if err := p.symbols.Get(ctx, name, &doc); err != nil || len(doc.Locs) == 0 {
		return nil, false
	}
	// Deterministic file order: locs accumulate in indexing order,
	// which varies across scans.
	sort.SliceStable(doc.Locs, func(i, j int) bool {
		return doc.Locs[i].URI < doc.Locs[j].URI
	})
	for _, spec := range symbolresolve.AllSpecs() {
		matches := matchesForSpec(spec, doc.Locs, len(parts) == 3, name)
		if len(matches) == 0 {
			continue
		}
		if progress != nil {
			progress.Report("Resolved from index", len(matches), 1, 1)
		}
		if len(matches) > 1 && len(parts) == 2 && spec.ImportPathQuery != "" {
			matches = p.dedupByImport(ctx, matches, parts[0])
		}
		if len(matches) > 1 {
			disambiguate(spec, matches, name)
		}
		return matches, true
	}
	return nil, false
}

// matchesForSpec converts the locations belonging to spec's language
// into matches, one per file. Two-part names prefer reference
// locations and fall back to definitions; three-part names use method
// definitions only.
func matchesForSpec(
	spec *symbolresolve.Spec, locs []symbolLoc, isMethod bool, name string,
) []syntaxapi.Match {
	pick := func(kind int) []syntaxapi.Match {
		var matches []syntaxapi.Match
		seen := make(map[string]bool)
		for _, l := range locs {
			if l.Kind != kind || seen[l.URI] ||
				symbolresolve.SpecForFile(l.URI) != spec {
				continue
			}
			seen[l.URI] = true
			matches = append(matches, syntaxapi.Match{
				URI:     l.URI,
				Pos:     term.Coordinates{X: l.X, Y: l.Y},
				Display: name,
			})
		}
		return matches
	}
	if isMethod {
		return pick(kindMethodDef)
	}
	if matches := pick(kindRef); len(matches) > 0 {
		return matches
	}
	return pick(kindDef)
}

// dedupByImport collapses matches that resolve pkg to the same import
// path according to each match file's stored alias map, annotating the
// surviving match with its import path. Files without a stored alias
// for pkg are keyed by their URI.
func (p *Parser) dedupByImport(
	ctx context.Context, matches []syntaxapi.Match, pkg string,
) []syntaxapi.Match {
	seen := make(map[string]bool)
	result := matches[:0]
	for _, m := range matches {
		key := m.URI
		var fd fileDoc
		if err := p.files.Get(ctx, m.URI, &fd); err == nil {
			if importPath, ok := fd.Imports[pkg]; ok {
				key = importPath
				m.ImportPath = importPath
			}
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, m)
	}
	return result
}

// disambiguate prefixes each match's Display name with its import path
// or display path, replicating the backing resolver's behavior when
// multiple matches survive.
func disambiguate(spec *symbolresolve.Spec, matches []syntaxapi.Match, name string) {
	for i, m := range matches {
		prefix := m.ImportPath
		if prefix == "" {
			prefix = m.URI
			if spec.DisplayPathFromURI != nil {
				prefix = spec.DisplayPathFromURI(m.URI)
			}
		}
		matches[i].Display = prefix + ": " + name
	}
}

// ListReferencedSymbols streams the indexed symbol names once a full
// scan has ever completed, passing through to the backing parser until
// then. Method-definition-only names are excluded, matching the
// backing parser's output. Names are served from their own partition
// of tiny marker docs: listing the symbols partition would decode
// every stored location of every symbol just to emit names.
//
// The iterator is a pure setup: the source — a partition scan the
// storage backend materializes in full, or the backing walk — is only
// opened on the first Next. Command completion constructs this
// iterator on the UI thread and pulls it from a background goroutine.
func (p *Parser) ListReferencedSymbols(
	_ context.Context,
) (iterator.Iterator[string], error) {
	var src iterator.Iterator[string]
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		if src == nil {
			it, err := p.openNameSource(ctx)
			if err != nil {
				return "", false, err
			}
			src = it
		}
		v, ok := src.Next(ctx)
		if !ok {
			return "", false, src.Err()
		}
		return v, true, nil
	}, func() error {
		if src == nil {
			return nil
		}
		return src.Close()
	}), nil
}

// openNameSource picks the symbol-name source: the names partition
// once a scan has ever completed, the backing parser's walk otherwise
// or when the partition cannot be listed.
func (p *Parser) openNameSource(
	ctx context.Context,
) (iterator.Iterator[string], error) {
	if !p.hasScannedEver() {
		return p.backing.ListReferencedSymbols(ctx)
	}
	it, err := p.names.List(ctx, nil)
	if err != nil {
		return p.backing.ListReferencedSymbols(ctx)
	}
	return iterator.FromFunc(func(context.Context) (string, bool, error) {
		if !it.HasNext() {
			return "", false, nil
		}
		var doc nameDoc
		if err := it.NextTo(&doc); err != nil {
			return "", false, err
		}
		return doc.Name, true, nil
	}, it.Close), nil
}

// run is the single writer goroutine: it performs the initial
// workspace scan and then drains the dirty set fed by Handle until the
// parser is closed.
func (p *Parser) run() {
	defer close(p.done)
	p.scan()
	p.setScanned()
	q := p.backing.NewQuerySession()
	defer func() { _ = q.Close() }()
	for {
		uri, ok := p.nextDirty()
		if ok {
			p.processURI(q, uri)
			continue
		}
		select {
		case <-p.wake:
		case <-p.ctx.Done():
			return
		}
	}
}

// scanJob is one file whose contributions must be rebuilt.
type scanJob struct {
	uri      workspaceapi.URI
	rel      string
	modTime  int64
	spec     *symbolresolve.Spec
	oldNames []string
	// force marks a record written by another schema version: its
	// derived records must be rewritten even where no state transition
	// is observed.
	force bool
}

type scanResult struct {
	scanJob
	ext symbolresolve.FileExtraction
	err error
}

type walkOutcome struct {
	scanned int
	err     error
}

// scan walks the workspace once, re-indexing only the files whose
// modification time differs from the persisted record, then drops
// records for files no longer on disk and persists the scan-complete
// marker. A restart over persisted state is stat-only. Extraction fans
// out across half the available cores — indexing is a background task
// and must not starve the editor — while every database write stays on
// this goroutine. Memory stays bounded per file: records are read and
// written one at a time, never accumulated into workspace-wide
// structures.
func (p *Parser) scan() {
	prog := p.newScanProgress()
	prog.start()
	scanned := 0
	defer func() { prog.done(scanned) }()

	jobs := make(chan scanJob)
	results := make(chan scanResult)
	var wg sync.WaitGroup
	workers := max(runtime.NumCPU()/2, 1)
	wg.Add(workers)
	for range workers {
		go debug.CapturePanicReport(func() {
			defer wg.Done()
			q := p.backing.NewQuerySession()
			defer func() { _ = q.Close() }()
			for j := range jobs {
				ext, err := symbolresolve.ExtractFile(p.ctx, q, j.spec, j.uri)
				select {
				case results <- scanResult{scanJob: j, ext: ext, err: err}:
				case <-p.ctx.Done():
					return
				}
			}
		})
	}
	walk := make(chan walkOutcome, 1)
	go debug.CapturePanicReport(func() {
		defer close(jobs)
		n, err := p.walkChanged(prog, jobs)
		walk <- walkOutcome{scanned: n, err: err}
	})
	go debug.CapturePanicReport(func() {
		wg.Wait()
		close(results)
	})

	// Bulk-loaded records are rebuildable, so their writes ask the bolt
	// backend to skip per-commit fsync; the completion marker below is
	// written synced, restoring durability for everything before it.
	wctx := bluebolt.ContextWithNoSync(p.ctx)
	for r := range results {
		if r.err != nil {
			if p.ctx.Err() != nil {
				continue
			}
			log.Errorf("symboldb: extract %q: %v", r.uri.String(), r.err)
			r.ext = symbolresolve.FileExtraction{}
		}
		p.applyExtraction(wctx, r.uri.String(), r.rel, r.modTime, r.ext, r.oldNames, r.force)
	}

	outcome := <-walk
	scanned = outcome.scanned
	if outcome.err != nil {
		// An incomplete walk must not wipe records for files it never
		// reached, nor claim the scan completed.
		log.Errorf("symboldb: scan %q: %v", p.root.String(), outcome.err)
		return
	}
	if p.ctx.Err() != nil {
		return
	}
	p.removeMissingFiles(wctx)
	if p.ctx.Err() != nil {
		return
	}
	marker := metaDoc{Complete: true, Version: schemaVersion}
	// Written with the plain context: this synced commit is the
	// durability barrier for every relaxed write above.
	if err := p.meta.Set(p.ctx, metaScanID, marker); err != nil {
		log.Errorf("symboldb: persist scan marker: %v", err)
	}
}

// walkChanged walks the workspace, reports per-directory progress and
// emits one job for every file whose stored record is stale. It reads
// file records concurrently with the scan goroutine's writes, which is
// safe because each file's record is only written after its job — the
// one this walk emits — has been applied.
func (p *Parser) walkChanged(prog *scanProgress, jobs chan<- scanJob) (int, error) {
	ctx := walkdir.WithContextFilter(p.ctx, p.walkFilter())
	it, err := walkdir.ListFiles(ctx, p.fs, ".")
	if err != nil {
		return 0, err
	}
	defer func() { _ = it.Close() }()

	scanned := 0
	// The walk runs on concurrent directory workers, so the stream hops
	// between directories; report each directory once.
	seenDirs := make(map[string]struct{})
	for {
		rel, ok := it.Next(ctx)
		if !ok {
			break
		}
		spec := symbolresolve.SpecForFile(rel)
		if spec == nil {
			continue
		}
		if dir := filepath.Dir(rel); !seen(seenDirs, dir) {
			prog.dir(dir, scanned)
		}
		scanned++
		uri, uerr := p.fs.URI(rel)
		if uerr != nil {
			continue
		}
		us := uri.String()
		finfo, serr := p.fs.Stat(rel)
		if serr != nil {
			continue
		}
		modTime := finfo.ModTime().UnixNano()
		var old fileDoc
		known := p.files.Get(p.ctx, us, &old) == nil
		if known && old.ModTime == modTime && old.Version == schemaVersion {
			continue
		}
		select {
		case jobs <- scanJob{
			uri: uri, rel: rel, modTime: modTime,
			spec: spec, oldNames: old.Names,
			force: known && old.Version != schemaVersion,
		}:
		case <-p.ctx.Done():
			return scanned, p.ctx.Err()
		}
	}
	return scanned, it.Err()
}

// removeMissingFiles streams the persisted file records and drops the
// contributions of every file that no longer exists on disk. Records
// are processed one at a time so a huge workspace's file table is
// never held in memory.
func (p *Parser) removeMissingFiles(ctx context.Context) {
	it, err := p.files.List(p.ctx, nil)
	if err != nil {
		log.Errorf("symboldb: list file records: %v", err)
		return
	}
	defer func() { _ = it.Close() }()
	for it.HasNext() {
		var doc fileDoc
		if err := it.NextTo(&doc); err != nil {
			log.Errorf("symboldb: read file record: %v", err)
			continue
		}
		if p.ctx.Err() != nil {
			return
		}
		if _, serr := p.fs.Stat(doc.Path); errors.Is(serr, fs.ErrNotExist) {
			p.removeFile(ctx, doc.URI, doc)
		}
	}
}

// seen records key in set, reporting whether it was already present.
func seen(set map[string]struct{}, key string) bool {
	if _, ok := set[key]; ok {
		return true
	}
	set[key] = struct{}{}
	return false
}

// scanProgress reports indexing progress. Notifications route through
// focus state and UI components that only the event loop may touch
// (see notisRouter.focusNotifications), so indexer goroutines enqueue
// closures through schedule instead of calling notify directly; id is
// accessed exclusively inside those closures, which the loop
// serializes. A dropped or failed start leaves id empty and the
// follow-up updates degrade to no-ops.
type scanProgress struct {
	notify   browserapi.Notifications
	schedule func(func()) bool
	id       string
}

func (p *Parser) newScanProgress() *scanProgress {
	return &scanProgress{notify: p.notify, schedule: p.schedule}
}

func (s *scanProgress) start() {
	s.schedule(func() {
		id, err := s.notify.Notify(browserapi.LevelInfo, "Indexing workspace symbols")
		if err == nil {
			s.id = id
		}
	})
}

// dir reports the directory currently being scanned. The total is
// unknown while the walk runs, so progress advances against a moving
// total that stays one ahead.
func (s *scanProgress) dir(dir string, scanned int) {
	s.schedule(func() {
		if s.id == "" {
			return
		}
		_ = s.notify.UpdateNotificationProgress(s.id,
			"Indexing symbols: "+dir, int64(scanned), int64(scanned)+1)
	})
}

// done completes the indexing notification; progress must reach total
// for the notification to settle.
func (s *scanProgress) done(scanned int) {
	s.schedule(func() {
		if s.id == "" {
			return
		}
		total := int64(max(scanned, 1))
		_ = s.notify.UpdateNotificationProgress(s.id,
			fmt.Sprintf("Indexed %d files", scanned), total, total)
	})
}

// processURI handles one dirty entry: a missing file drops its
// contributions, a changed one is re-indexed, an unchanged one is a
// no-op.
func (p *Parser) processURI(q symbolresolve.FileQueryer, us string) {
	uri, err := workspaceapi.ParseURI(us)
	if err != nil {
		return
	}
	rel := workspaceapi.RelPath(p.root, uri)
	spec := symbolresolve.SpecForFile(rel)
	if spec == nil {
		return
	}
	var old fileDoc
	known := p.files.Get(p.ctx, us, &old) == nil
	finfo, err := p.fs.Stat(rel)
	if err != nil {
		if known {
			p.removeFile(p.ctx, us, old)
		}
		return
	}
	modTime := finfo.ModTime().UnixNano()
	if known && old.ModTime == modTime && old.Version == schemaVersion {
		return
	}
	ext, err := symbolresolve.ExtractFile(p.ctx, q, spec, uri)
	if err != nil {
		if p.ctx.Err() != nil {
			return
		}
		log.Errorf("symboldb: extract %q: %v", us, err)
		ext = symbolresolve.FileExtraction{}
	}
	p.applyExtraction(p.ctx, us, rel, modTime, ext, old.Names,
		known && old.Version != schemaVersion)
}

// applyExtraction replaces one file's contributions in the database.
// Callers pass an empty extraction for unparseable files so they do
// not retain stale entries. ctx carries the writes' durability mode:
// the initial scan relaxes it, event-driven updates stay synced.
func (p *Parser) applyExtraction(
	ctx context.Context, us, rel string, modTime int64,
	ext symbolresolve.FileExtraction, oldNames []string, force bool,
) {
	symbols := make(map[string][]symbolLoc)
	for _, s := range ext.Symbols {
		symbols[s.Name] = append(symbols[s.Name], symbolLoc{
			URI: us, X: s.Pos.X, Y: s.Pos.Y, Kind: storedKind(s.Kind),
		})
	}
	for _, name := range oldNames {
		if _, ok := symbols[name]; ok {
			continue
		}
		p.updateSymbol(ctx, name, us, nil, force)
	}
	for name, locs := range symbols {
		p.updateSymbol(ctx, name, us, locs, force)
	}
	doc := fileDoc{
		URI:     us,
		Path:    rel,
		ModTime: modTime,
		Version: schemaVersion,
		Names:   slices.Sorted(maps.Keys(symbols)),
		Imports: ext.Imports,
	}
	if err := p.files.Set(ctx, us, doc); err != nil {
		log.Errorf("symboldb: persist file record %q: %v", us, err)
	}
}

// removeFile drops every contribution recorded for a file and deletes
// its record.
func (p *Parser) removeFile(ctx context.Context, us string, old fileDoc) {
	for _, name := range old.Names {
		p.updateSymbol(ctx, name, us, nil, false)
	}
	if err := p.files.Delete(ctx, us); err != nil {
		log.Errorf("symboldb: delete file record %q: %v", us, err)
	}
}

// updateSymbol replaces the locations one file contributes to a symbol
// doc, deleting the doc when no locations remain. The names partition
// is kept in step: a marker is written or removed when the symbol's
// listed state transitions, and rewritten unconditionally under force,
// when the caller is reprocessing a record from another schema version
// whose derived state cannot be trusted. Writes whose outcome is
// already stored are skipped: on the production backend every write is
// an fsync'd transaction costing milliseconds, while reads are memory
// lookups.
func (p *Parser) updateSymbol(
	ctx context.Context, name, us string, locs []symbolLoc, force bool,
) {
	var doc symbolDoc
	err := p.symbols.Get(ctx, name, &doc)
	if err != nil && !errors.Is(err, storageapi.ErrNotFound) {
		log.Errorf("symboldb: read symbol %q: %v", name, err)
		return
	}
	known := err == nil
	wasListed := listed(doc.Locs)
	kept := make([]symbolLoc, 0, len(doc.Locs)+len(locs))
	for _, l := range doc.Locs {
		if l.URI != us {
			kept = append(kept, l)
		}
	}
	kept = append(kept, locs...)
	if len(kept) == 0 {
		if !known {
			return
		}
		if err := p.symbols.Delete(ctx, name); err != nil {
			log.Errorf("symboldb: delete symbol %q: %v", name, err)
		}
		if wasListed || (force && p.hasName(ctx, name)) {
			p.deleteName(ctx, name)
		}
		return
	}
	if !sameLocs(doc.Locs, kept) {
		if err := p.symbols.Set(ctx, name, symbolDoc{Name: name, Locs: kept}); err != nil {
			log.Errorf("symboldb: persist symbol %q: %v", name, err)
		}
	}
	switch isListed := listed(kept); {
	case isListed && (!wasListed || (force && !p.hasName(ctx, name))):
		if err := p.names.Set(ctx, name, nameDoc{Name: name}); err != nil {
			log.Errorf("symboldb: persist name %q: %v", name, err)
		}
	case wasListed && !isListed:
		p.deleteName(ctx, name)
	}
}

// sameLocs reports whether two location slices hold the same multiset
// of locations: rebuilding a file's contribution moves its locations
// to the tail of the merged slice without changing the contents.
func sameLocs(a, b []symbolLoc) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[symbolLoc]int, len(a))
	for _, l := range a {
		counts[l]++
	}
	for _, l := range b {
		if counts[l] == 0 {
			return false
		}
		counts[l]--
	}
	return true
}

// hasName reports whether a symbol's listed marker is already stored.
func (p *Parser) hasName(ctx context.Context, name string) bool {
	var doc nameDoc
	return p.names.Get(ctx, name, &doc) == nil
}

// deleteName removes a symbol's listed marker, tolerating markers that
// never existed.
func (p *Parser) deleteName(ctx context.Context, name string) {
	if err := p.names.Delete(ctx, name); err != nil &&
		!errors.Is(err, storageapi.ErrNotFound) {
		log.Errorf("symboldb: delete name %q: %v", name, err)
	}
}

// walkFilter prunes gitignored entries and hidden directories from the
// workspace walk, matching the backing parser's walk behavior. Failure
// to load the gitignore matcher degrades to hidden-directory pruning.
func (p *Parser) walkFilter() walkdir.Filter {
	hidden := hiddenDirMatcher{vctrl.HiddenBaseMatcher()}
	m, err := vctrl.LoadGitignore(p.fs)
	if err != nil {
		return hidden
	}
	return vctrl.AnyMatcher(m, hidden)
}

// hiddenDirMatcher restricts a basename-hidden matcher to directories
// so hidden source files at visible paths are still scanned while
// hidden directories like .git or .venv are pruned.
type hiddenDirMatcher struct{ m vctrl.Matcher }

func (h hiddenDirMatcher) Match(uri workspaceapi.URI, isDir bool) bool {
	return isDir && h.m.Match(uri, isDir)
}

func (h hiddenDirMatcher) MatchRelPath(relpath string, isDir bool) bool {
	return isDir && h.m.MatchRelPath(relpath, isDir)
}
