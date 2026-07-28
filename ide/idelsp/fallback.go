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

package idelsp

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/search"
)

// isNoServer reports whether err means no language server can serve
// the request, which is the only condition under which the
// tree-sitter fallback is consulted.
func isNoServer(err error) bool {
	return errors.Is(err, ErrNoServer) ||
		errors.Is(err, ErrLanguageNotSupported)
}

const (
	captureScope     = "local.scope"
	captureReference = "local.reference"
	// captureDefinition is a dot-terminated prefix matching every
	// local.definition.* sub-kind without also matching lookalikes.
	captureDefinition = "local.definition."
)

const (
	// maxFallbackCaptures bounds the locals.scm captures drained for
	// a single file so a pathological file cannot pin the request.
	maxFallbackCaptures = 100000
	// maxFallbackLocations bounds cross-file candidate locations.
	maxFallbackLocations = 50
	// maxWorkspaceSymbolResults bounds fuzzy workspace symbol hits.
	maxWorkspaceSymbolResults = 20
	// maxIndexedNames bounds how many indexed symbol names are
	// enumerated for unqualified and fuzzy lookups.
	maxIndexedNames = 100000
)

// lspFallback is the Manager's syntax-backed answer to requests that
// no language server can serve. It is always non-nil on a Manager:
// noFallback is installed when no parser is configured.
type lspFallback interface {
	definition(ctx context.Context, params semanticapi.DefinitionParams,
	) (semanticapi.LocationResult, error)
	references(ctx context.Context, params semanticapi.ReferenceParams,
	) ([]semanticapi.Location, error)
	workspaceSymbol(ctx context.Context, params semanticapi.WorkspaceSymbolParams,
	) ([]semanticapi.SymbolInformation, error)
	documentSymbol(ctx context.Context, params semanticapi.DocumentSymbolParams,
	) (semanticapi.DocumentSymbolResult, error)
}

// errNoFallbackParser makes Manager keep the original no-server
// error when no fallback parser is configured.
var errNoFallbackParser = errors.New("no fallback parser configured")

// noFallback is the robust stand-in when Config.Parser is absent.
type noFallback struct{}

func (noFallback) definition(
	context.Context, semanticapi.DefinitionParams,
) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, errNoFallbackParser
}

func (noFallback) references(
	context.Context, semanticapi.ReferenceParams,
) ([]semanticapi.Location, error) {
	return nil, errNoFallbackParser
}

func (noFallback) workspaceSymbol(
	context.Context, semanticapi.WorkspaceSymbolParams,
) ([]semanticapi.SymbolInformation, error) {
	return nil, nil
}

func (noFallback) documentSymbol(
	context.Context, semanticapi.DocumentSymbolParams,
) (semanticapi.DocumentSymbolResult, error) {
	return semanticapi.DocumentSymbolResult{}, errNoFallbackParser
}

// syntaxFallback answers a subset of LSP requests from tree-sitter
// when no language server is available for a file's language. Local
// symbols are resolved through locals.scm scope/definition captures;
// cross-file symbols through the parser's qualified symbol index.
type syntaxFallback struct {
	parser   syntaxapi.Parser
	indexed  bool
	readFile func(path string) (string, error)
}

var (
	_ lspFallback = (*syntaxFallback)(nil)
	_ lspFallback = noFallback{}
)

// newSyntaxFallback builds a syntax-backed fallback. Dependencies are
// mandatory: passing nil is a programming error and panics so a
// misconfigured production build fails loudly instead of silently
// degrading. indexed reports that parser is backed by a symbol index,
// making workspace-wide symbol enumeration cheap enough to serve
// WorkspaceSymbol and unqualified lookups.
func newSyntaxFallback(
	parser syntaxapi.Parser,
	readFile func(path string) (string, error),
	indexed bool,
) *syntaxFallback {
	if parser == nil {
		panic("syntaxFallback: nil parser")
	}
	if readFile == nil {
		panic("syntaxFallback: nil readFile")
	}
	return &syntaxFallback{
		parser:   parser,
		indexed:  indexed,
		readFile: readFile,
	}
}

// outlineNodeCaptures asks only for what an outline needs; reference
// captures would dominate the result set without contributing.
const outlineNodeCaptures = syntaxapi.NodeCaptureScope |
	syntaxapi.NodeCaptureDefinitionFunc |
	syntaxapi.NodeCaptureDefinitionVar |
	syntaxapi.NodeCaptureDefinitionMethod |
	syntaxapi.NodeCaptureDefinitionType |
	syntaxapi.NodeCaptureDefinitionNamespace

const fallbackNodeCaptures = outlineNodeCaptures |
	syntaxapi.NodeCaptureReference

func (s *syntaxFallback) definition(
	ctx context.Context, params semanticapi.DefinitionParams,
) (semanticapi.LocationResult, error) {
	uri, err := workspaceapi.ParseURI(params.TextDocument.URI)
	if err != nil {
		return semanticapi.LocationResult{}, err
	}
	pos := positionToCoord(params.Position)
	caps, err := s.localCaptures(ctx, uri, fallbackNodeCaptures)
	if err != nil {
		return semanticapi.LocationResult{}, err
	}
	target, ok := captureAt(caps, pos)
	if ok && isDefinitionCapture(target) {
		loc := resultLocation(params.TextDocument.URI, target)
		return semanticapi.LocationResult{Location: &loc}, nil
	}
	tokens := s.lazyToken(uri, params.Position)
	name := ""
	if ok {
		name = target.Text
	}
	if name == "" {
		// locals.scm did not capture the node under the cursor
		// (e.g. a member identifier); fall back to text extraction.
		_, name = tokens()
	}
	if name != "" {
		if def, found := resolveLocalDef(caps, name, pos); found {
			loc := resultLocation(params.TextDocument.URI, def)
			return semanticapi.LocationResult{Location: &loc}, nil
		}
	}
	if token, _ := tokens(); strings.Contains(token, ".") {
		if locs := s.resolveQualified(ctx, token); len(locs) > 0 {
			return semanticapi.LocationResult{Locations: locs}, nil
		}
	}
	if name != "" && s.indexed {
		if locs := s.resolveUnqualified(ctx, name); len(locs) > 0 {
			return semanticapi.LocationResult{Locations: locs}, nil
		}
	}
	return semanticapi.LocationResult{}, nil
}

func (s *syntaxFallback) references(
	ctx context.Context, params semanticapi.ReferenceParams,
) ([]semanticapi.Location, error) {
	uri, err := workspaceapi.ParseURI(params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	pos := positionToCoord(params.Position)
	caps, err := s.localCaptures(ctx, uri, fallbackNodeCaptures)
	if err != nil {
		return nil, err
	}
	target, ok := captureAt(caps, pos)
	tokens := s.lazyToken(uri, params.Position)
	name := ""
	if ok {
		name = target.Text
	}
	if name == "" {
		_, name = tokens()
	}
	var def syntaxapi.Result
	haveDef := false
	if ok && isDefinitionCapture(target) {
		def, haveDef = target, true
	} else if name != "" {
		def, haveDef = resolveLocalDef(caps, name, pos)
	}
	if haveDef {
		return localReferences(
			params.TextDocument.URI, caps, name, def,
			params.Context.IncludeDeclaration,
		), nil
	}
	if token, _ := tokens(); strings.Contains(token, ".") {
		if locs := s.resolveQualified(ctx, token); len(locs) > 0 {
			return locs, nil
		}
	}
	return nil, nil
}

func (s *syntaxFallback) workspaceSymbol(
	ctx context.Context, params semanticapi.WorkspaceSymbolParams,
) ([]semanticapi.SymbolInformation, error) {
	if !s.indexed {
		// Enumerating symbols without an index means a workspace
		// scan per request; keep the zero-server contract instead.
		return nil, nil
	}
	names, err := s.indexedNames(ctx)
	if err != nil {
		return nil, err
	}
	input := make([][]byte, len(names))
	for i, n := range names {
		input[i] = []byte(n)
	}
	matches := search.Fuzzy(input, params.Query, false)
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].Score() > matches[j].Score()
	})
	var ret []semanticapi.SymbolInformation
	for _, m := range matches {
		if len(ret) >= maxWorkspaceSymbolResults {
			break
		}
		name := names[m.Index()]
		locs := s.resolveName(ctx, name, 1)
		if len(locs) == 0 {
			continue
		}
		ret = append(ret, semanticapi.SymbolInformation{
			Name:     name,
			Kind:     kindForName(name),
			Location: locs[0],
		})
	}
	return ret, nil
}

// documentSymbol builds a file outline from locals.scm captures.
// Definitions that introduce a scope (functions, methods, types)
// become containers for every definition nested inside that scope.
func (s *syntaxFallback) documentSymbol(
	ctx context.Context, params semanticapi.DocumentSymbolParams,
) (semanticapi.DocumentSymbolResult, error) {
	uri, err := workspaceapi.ParseURI(params.TextDocument.URI)
	if err != nil {
		return semanticapi.DocumentSymbolResult{}, err
	}
	caps, err := s.localCaptures(ctx, uri, outlineNodeCaptures)
	if err != nil {
		return semanticapi.DocumentSymbolResult{}, err
	}
	syms := outlineSymbols(caps)
	if len(syms) == 0 {
		return semanticapi.DocumentSymbolResult{}, nil
	}
	return semanticapi.DocumentSymbolResult{DocumentSymbols: syms}, nil
}

// localCaptures drains one locals.scm pass over uri, yielding every
// requested capture in the file.
func (s *syntaxFallback) localCaptures(
	ctx context.Context, uri workspaceapi.URI,
	nodeTypes syntaxapi.NodeCaptureName,
) ([]syntaxapi.Result, error) {
	it, err := s.parser.QueryNode(uri, nodeTypes)
	if err != nil {
		return nil, err
	}
	defer it.Close() // nolint:errcheck
	var ret []syntaxapi.Result
	for len(ret) < maxFallbackCaptures {
		v, ok := it.Next(ctx)
		if !ok {
			break
		}
		ret = append(ret, v)
	}
	if err := it.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

// resolveQualified resolves a dotted token through the parser's
// symbol index, retrying progressively shorter dotted suffixes so
// both fully and partially qualified spellings hit index entries.
func (s *syntaxFallback) resolveQualified(
	ctx context.Context, token string,
) []semanticapi.Location {
	parts := strings.Split(token, ".")
	for i := 0; i < len(parts)-1; i++ {
		name := strings.Join(parts[i:], ".")
		if locs := s.resolveName(ctx, name, maxFallbackLocations); len(locs) > 0 {
			return locs
		}
	}
	return nil
}

// resolveUnqualified finds indexed qualified names whose last segment
// equals name and returns all their locations, letting the caller
// present every namespace candidate.
func (s *syntaxFallback) resolveUnqualified(
	ctx context.Context, name string,
) []semanticapi.Location {
	names, err := s.indexedNames(ctx)
	if err != nil {
		return nil
	}
	var ret []semanticapi.Location
	for _, n := range names {
		if n != name && !strings.HasSuffix(n, "."+name) {
			continue
		}
		ret = append(ret, s.resolveName(
			ctx, n, maxFallbackLocations-len(ret),
		)...)
		if len(ret) >= maxFallbackLocations {
			break
		}
	}
	return ret
}

func (s *syntaxFallback) resolveName(
	ctx context.Context, name string, limit int,
) []semanticapi.Location {
	it, err := s.parser.ResolveSymbol(ctx, name, nil)
	if err != nil {
		return nil
	}
	defer it.Close() // nolint:errcheck
	var ret []semanticapi.Location
	for len(ret) < limit {
		m, ok := it.Next(ctx)
		if !ok {
			break
		}
		pos := semanticapi.Position{
			Line:      uint32(m.Pos.Y),
			Character: uint32(m.Pos.X),
		}
		ret = append(ret, semanticapi.Location{
			URI:   m.URI,
			Range: semanticapi.Range{Start: pos, End: pos},
		})
	}
	return ret
}

func (s *syntaxFallback) indexedNames(
	ctx context.Context,
) ([]string, error) {
	it, err := s.parser.ListReferencedSymbols(ctx)
	if err != nil {
		return nil, err
	}
	defer it.Close() // nolint:errcheck
	var ret []string
	for len(ret) < maxIndexedNames {
		n, ok := it.Next(ctx)
		if !ok {
			break
		}
		ret = append(ret, n)
	}
	return ret, it.Err()
}

// lazyToken defers the source-line read behind a memoized closure so
// requests resolved purely from locals.scm captures never touch the
// file contents.
func (s *syntaxFallback) lazyToken(
	uri workspaceapi.URI, pos semanticapi.Position,
) func() (token, segment string) {
	done := false
	var token, segment string
	return func() (string, string) {
		if !done {
			token, segment = s.tokenAt(uri, pos)
			done = true
		}
		return token, segment
	}
}

// tokenAt extracts the dotted token spanning the cursor from the
// source line, truncated after the segment under the cursor, plus
// that segment. Both are empty when the cursor is not on a token.
func (s *syntaxFallback) tokenAt(
	uri workspaceapi.URI, pos semanticapi.Position,
) (token, segment string) {
	content, err := s.readFile(uri.Path())
	if err != nil {
		return "", ""
	}
	lines := strings.Split(content, "\n")
	if int(pos.Line) >= len(lines) {
		return "", ""
	}
	line := []rune(lines[pos.Line])
	i := int(pos.Character)
	if i >= len(line) || !isTokenRune(line[i]) {
		if i > 0 && i-1 < len(line) && isTokenRune(line[i-1]) {
			i--
		} else {
			return "", ""
		}
	}
	start := i
	for start > 0 && isTokenRune(line[start-1]) {
		start--
	}
	segStart, segEnd := i, i
	for segStart > start && isWordRune(line[segStart-1]) {
		segStart--
	}
	for segEnd < len(line) && isWordRune(line[segEnd]) {
		segEnd++
	}
	token = strings.Trim(string(line[start:segEnd]), ".")
	return token, string(line[segStart:segEnd])
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isTokenRune(r rune) bool {
	return isWordRune(r) || r == '.'
}

func isScopeCapture(r syntaxapi.Result) bool {
	return r.CaptureName == captureScope
}

func isReferenceCapture(r syntaxapi.Result) bool {
	return r.CaptureName == captureReference
}

func isDefinitionCapture(r syntaxapi.Result) bool {
	return strings.HasPrefix(r.CaptureName, captureDefinition)
}

// captureAt returns the smallest non-scope capture containing pos. A
// caret sitting immediately after the last rune of an identifier
// still counts, matching editor word-under-cursor conventions.
func captureAt(
	caps []syntaxapi.Result, pos term.Coordinates,
) (syntaxapi.Result, bool) {
	if r, ok := captureContaining(caps, pos); ok {
		return r, true
	}
	if pos.X > 0 {
		return captureContaining(
			caps, term.Coordinates{X: pos.X - 1, Y: pos.Y},
		)
	}
	return syntaxapi.Result{}, false
}

func captureContaining(
	caps []syntaxapi.Result, pos term.Coordinates,
) (syntaxapi.Result, bool) {
	var best syntaxapi.Result
	found := false
	for _, c := range caps {
		if isScopeCapture(c) || !coordInRange(pos, c.From, c.To) {
			continue
		}
		if !found || rangeWithin(c.From, c.To, best.From, best.To) {
			best = c
			found = true
		}
	}
	return best, found
}

// resolveLocalDef resolves name at pos against the file's definition
// captures: a definition is visible when its innermost enclosing
// scope contains pos, the definition in the narrowest such scope wins
// (shadowing), and within one scope the nearest definition at or
// before pos wins.
func resolveLocalDef(
	caps []syntaxapi.Result, name string, pos term.Coordinates,
) (syntaxapi.Result, bool) {
	var scopes []syntaxapi.Result
	for _, c := range caps {
		if isScopeCapture(c) {
			scopes = append(scopes, c)
		}
	}
	var best, bestScope syntaxapi.Result
	found, bestScoped := false, false
	for _, d := range caps {
		if !isDefinitionCapture(d) || d.Text != name {
			continue
		}
		ds, scoped := innermostScope(scopes, d.From)
		if scoped && liftsScope(d) {
			// A function/type name sits inside the very node that
			// forms its scope, yet belongs to the enclosing scope
			// (classic locals.scm hoisting).
			ds, scoped = innermostScopeExcluding(scopes, d.From, ds)
		}
		if scoped && !coordInRange(pos, ds.From, ds.To) {
			continue
		}
		switch {
		case !found:
		case scopeNarrower(ds, scoped, bestScope, bestScoped):
		case scopeNarrower(bestScope, bestScoped, ds, scoped):
			continue
		case !preferredDef(d, best, pos):
			continue
		}
		best, bestScope = d, ds
		found, bestScoped = true, scoped
	}
	return best, found
}

// localReferences collects the reference captures whose own
// resolution lands on def, which makes shadowed same-name sites drop
// out by construction.
func localReferences(
	uri string, caps []syntaxapi.Result, name string,
	def syntaxapi.Result, includeDeclaration bool,
) []semanticapi.Location {
	var ret []semanticapi.Location
	if includeDeclaration {
		ret = append(ret, resultLocation(uri, def))
	}
	for _, r := range caps {
		if !isReferenceCapture(r) || r.Text != name {
			continue
		}
		if r.From == def.From {
			continue
		}
		rd, ok := resolveLocalDef(caps, name, r.From)
		if !ok || rd.From != def.From || rd.To != def.To {
			continue
		}
		ret = append(ret, resultLocation(uri, r))
	}
	return ret
}

func innermostScope(
	scopes []syntaxapi.Result, pos term.Coordinates,
) (syntaxapi.Result, bool) {
	return innermostScopeExcluding(scopes, pos, syntaxapi.Result{
		From: term.Coordinates{X: -1, Y: -1},
		To:   term.Coordinates{X: -1, Y: -1},
	})
}

func innermostScopeExcluding(
	scopes []syntaxapi.Result, pos term.Coordinates,
	excluded syntaxapi.Result,
) (syntaxapi.Result, bool) {
	var best syntaxapi.Result
	found := false
	for _, sc := range scopes {
		if sc.From == excluded.From && sc.To == excluded.To {
			continue
		}
		if !coordInRange(pos, sc.From, sc.To) {
			continue
		}
		if !found || rangeWithin(sc.From, sc.To, best.From, best.To) {
			best = sc
			found = true
		}
	}
	return best, found
}

// liftsScope reports whether the definition's visibility is hoisted
// one scope up because the captured name lives inside the scope node
// the definition itself introduces.
func liftsScope(d syntaxapi.Result) bool {
	switch d.CaptureName {
	case "local.definition.function", "local.definition.method",
		"local.definition.type", "local.definition.namespace":
		return true
	}
	return false
}

// scopeNarrower reports whether scope a is strictly narrower than
// scope b. A definition without an enclosing scope capture is
// file-level, so any scoped definition shadows it.
func scopeNarrower(
	a syntaxapi.Result, aScoped bool,
	b syntaxapi.Result, bScoped bool,
) bool {
	if aScoped && !bScoped {
		return true
	}
	if !aScoped {
		return false
	}
	if a.From == b.From && a.To == b.To {
		return false
	}
	return rangeWithin(a.From, a.To, b.From, b.To)
}

// preferredDef reports whether candidate d beats best within the same
// scope: the latest definition at or before pos wins, and a
// definition before pos always beats one after it.
func preferredDef(d, best syntaxapi.Result, pos term.Coordinates) bool {
	dBefore := coordBeforeOrAt(d.From, pos)
	bestBefore := coordBeforeOrAt(best.From, pos)
	if dBefore != bestBefore {
		return dBefore
	}
	if dBefore {
		return coordBeforeOrAt(best.From, d.From)
	}
	return coordBeforeOrAt(d.From, best.From)
}

func coordBeforeOrAt(a, b term.Coordinates) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.X <= b.X
}

// coordInRange reports whether p lies within [from, to)
// (line/column ordering, end-exclusive).
func coordInRange(p, from, to term.Coordinates) bool {
	if p.Y < from.Y || p.Y > to.Y {
		return false
	}
	if p.Y == from.Y && p.X < from.X {
		return false
	}
	if p.Y == to.Y && p.X >= to.X {
		return false
	}
	return true
}

// rangeWithin reports whether [innerFrom, innerTo) is contained
// within [outerFrom, outerTo).
func rangeWithin(
	innerFrom, innerTo, outerFrom, outerTo term.Coordinates,
) bool {
	if !coordInRange(innerFrom, outerFrom, outerTo) {
		return false
	}
	if innerTo.Y < outerTo.Y {
		return true
	}
	return innerTo.Y == outerTo.Y && innerTo.X <= outerTo.X
}

func positionToCoord(p semanticapi.Position) term.Coordinates {
	return term.Coordinates{X: int(p.Character), Y: int(p.Line)}
}

func resultLocation(
	uri string, r syntaxapi.Result,
) semanticapi.Location {
	return semanticapi.Location{URI: uri, Range: resultRange(r)}
}

func resultRange(r syntaxapi.Result) semanticapi.Range {
	return semanticapi.Range{
		Start: coordToPosition(r.From),
		End:   coordToPosition(r.To),
	}
}

func coordToPosition(c term.Coordinates) semanticapi.Position {
	return semanticapi.Position{
		Line:      uint32(c.Y),
		Character: uint32(c.X),
	}
}

// kindForName approximates an LSP symbol kind from a qualified name:
// the index stores functions, types and methods, and only methods
// carry a container segment between qualifier and name.
func kindForName(name string) semanticapi.SymbolKind {
	if strings.Count(name, ".") >= 2 {
		return semanticapi.SymbolKindMethod
	}
	return semanticapi.SymbolKindFunction
}

type rangeKey struct{ from, to term.Coordinates }

func rangeKeyOf(r syntaxapi.Result) rangeKey {
	return rangeKey{from: r.From, to: r.To}
}

// outlineSymbols nests each definition capture under the definition
// owning its enclosing scope, producing a document symbol tree.
func outlineSymbols(caps []syntaxapi.Result) []semanticapi.DocumentSymbol {
	var scopes, defs []syntaxapi.Result
	for _, c := range caps {
		switch {
		case isScopeCapture(c):
			scopes = append(scopes, c)
		case isDefinitionCapture(c) && c.Text != "":
			defs = append(defs, c)
		}
	}
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].From != defs[j].From {
			return coordBefore(defs[i].From, defs[j].From)
		}
		return capturePriority(defs[i].CaptureName) <
			capturePriority(defs[j].CaptureName)
	})
	defs = dedupeDefs(defs)
	root := widestScope(scopes)
	own := make([]syntaxapi.Result, len(defs))
	owns := make([]bool, len(defs))
	ownerOf := make(map[rangeKey]int, len(defs))
	parent := make([]syntaxapi.Result, len(defs))
	parented := make([]bool, len(defs))
	for i, d := range defs {
		sc, ok := innermostScope(scopes, d.From)
		if _, taken := ownerOf[rangeKeyOf(sc)]; ok && !taken && ownsScope(d, sc, root) {
			own[i], owns[i] = sc, true
			ownerOf[rangeKeyOf(sc)] = i
			parent[i], parented[i] = innermostScopeExcluding(scopes, d.From, sc)
			continue
		}
		parent[i], parented[i] = sc, ok
	}
	children := make([][]int, len(defs))
	var roots []int
	for i := range defs {
		p, ok := nearestOwner(scopes, ownerOf, parent[i], parented[i])
		if !ok || p == i {
			roots = append(roots, i)
			continue
		}
		children[p] = append(children[p], i)
	}
	return outlineTree(defs, own, owns, children, roots)
}

func outlineTree(
	defs, own []syntaxapi.Result, owns []bool,
	children [][]int, idxs []int,
) []semanticapi.DocumentSymbol {
	if len(idxs) == 0 {
		return nil
	}
	ret := make([]semanticapi.DocumentSymbol, 0, len(idxs))
	for _, i := range idxs {
		d := defs[i]
		full := resultRange(d)
		if owns[i] {
			full = spanRange(d, own[i])
		}
		ret = append(ret, semanticapi.DocumentSymbol{
			Name:           d.Text,
			Kind:           kindForCapture(d.CaptureName),
			Range:          full,
			SelectionRange: resultRange(d),
			Children: outlineTree(
				defs, own, owns, children, children[i],
			),
		})
	}
	return ret
}

// ownsScope reports whether d introduces sc. locals.scm hoists a
// function or type name out of the very node forming its scope, so
// that scope node starts on the definition's own line. The file-root
// scope is never owned: a definition on the first line would
// otherwise swallow the whole outline.
func ownsScope(d, sc, root syntaxapi.Result) bool {
	if !liftsScope(d) || rangeKeyOf(sc) == rangeKeyOf(root) {
		return false
	}
	return sc.From.Y == d.From.Y
}

// nearestOwner walks outwards from sc to the closest enclosing scope
// that some definition owns.
func nearestOwner(
	scopes []syntaxapi.Result, ownerOf map[rangeKey]int,
	sc syntaxapi.Result, ok bool,
) (int, bool) {
	for range scopes {
		if !ok {
			break
		}
		if idx, found := ownerOf[rangeKeyOf(sc)]; found {
			return idx, true
		}
		sc, ok = enclosingScope(scopes, sc)
	}
	return 0, false
}

// enclosingScope returns the narrowest scope strictly containing sc.
func enclosingScope(
	scopes []syntaxapi.Result, sc syntaxapi.Result,
) (syntaxapi.Result, bool) {
	var best syntaxapi.Result
	found := false
	for _, c := range scopes {
		if rangeKeyOf(c) == rangeKeyOf(sc) ||
			!rangeWithin(sc.From, sc.To, c.From, c.To) {
			continue
		}
		if !found || rangeWithin(c.From, c.To, best.From, best.To) {
			best, found = c, true
		}
	}
	return best, found
}

func widestScope(scopes []syntaxapi.Result) syntaxapi.Result {
	var best syntaxapi.Result
	found := false
	for _, sc := range scopes {
		if !found || rangeWithin(best.From, best.To, sc.From, sc.To) {
			best, found = sc, true
		}
	}
	return best
}

// dedupeDefs drops definition captures that repeat a range already
// taken: a single declaration can match several locals.scm patterns
// (python captures a method as both method and function) and the
// duplicate would otherwise nest under itself.
func dedupeDefs(defs []syntaxapi.Result) []syntaxapi.Result {
	seen := make(map[rangeKey]bool, len(defs))
	ret := defs[:0]
	for _, d := range defs {
		key := rangeKeyOf(d)
		if seen[key] {
			continue
		}
		seen[key] = true
		ret = append(ret, d)
	}
	return ret
}

// capturePriority orders the capture kinds competing for one range,
// most specific first, so dedupeDefs keeps the best description.
func capturePriority(capture string) int {
	switch capture {
	case "local.definition.method":
		return 0
	case "local.definition.type":
		return 1
	case "local.definition.function":
		return 2
	case "local.definition.namespace":
		return 3
	default:
		return 4
	}
}

func spanRange(a, b syntaxapi.Result) semanticapi.Range {
	from, to := a.From, a.To
	if coordBefore(b.From, from) {
		from = b.From
	}
	if coordBefore(to, b.To) {
		to = b.To
	}
	return semanticapi.Range{
		Start: coordToPosition(from),
		End:   coordToPosition(to),
	}
}

func coordBefore(a, b term.Coordinates) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.X < b.X
}

func kindForCapture(capture string) semanticapi.SymbolKind {
	switch capture {
	case "local.definition.function":
		return semanticapi.SymbolKindFunction
	case "local.definition.method":
		return semanticapi.SymbolKindMethod
	case "local.definition.type":
		return semanticapi.SymbolKindClass
	case "local.definition.namespace":
		return semanticapi.SymbolKindNamespace
	default:
		return semanticapi.SymbolKindVariable
	}
}
