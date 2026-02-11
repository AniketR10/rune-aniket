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

package syntax

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/sirupsen/logrus"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace/walkdir"
)

// NewSearcher returns a workspace-wide syntax searcher.
func NewSearcher(
	w workspaceapi.FileSystem, pkg PkgManager, uri workspaceapi.URI,
) syntaxapi.Searcher {
	return searcher{w: w, uri: uri, pkg: pkg}
}

var (
	defaultWorkers = runtime.NumCPU()
)

type searcher struct {
	w   workspaceapi.FileSystem
	pkg PkgManager
	uri workspaceapi.URI
}

func (s searcher) Search(query string, captureNames []string) (
	iterator.Iterator[syntaxapi.Result], error,
) {
	ctx, cancel := context.WithCancel(context.Background())
	paths, err := walkdir.ListFiles(ctx, s.w, ".")
	if err != nil {
		cancel()
		return nil, err
	}

	files := make(chan string)
	results := make(chan syntaxapi.Result)
	closeWaitCh := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(defaultWorkers)
	errs := make([]error, defaultWorkers)
	validErrors := make([]map[string]*expectedError, defaultWorkers)
	for i := range defaultWorkers {
		validErrors[i] = make(map[string]*expectedError)
		go func(err *error, expectedErrors map[string]*expectedError) {
			defer wg.Done()
			readSymbolsWorker(ctx, s.w, s.pkg, s.uri, query, results, files, err,
				expectedErrors, captureNames)
		}(&errs[i], validErrors[i])
	}

	it := &listSymbolsIterator{ctx: ctx, ch: results}
	it.cancel = cancel
	it.closeWaitCh = closeWaitCh

	var itErr error
	go debug.CapturePanicReport(func() {
		defer close(closeWaitCh)
		defer close(results)
		defer paths.Close()

		for {
			file, ok := paths.Next(ctx)
			if !ok {
				break
			}
			select {
			case files <- file:
				continue
			case <-ctx.Done():
			}
			break
		}
		if err := paths.Err(); err != nil {
			itErr = err
		}
		close(files)
		wg.Wait()

		it.mu.Lock()
		defer it.mu.Unlock()

		it.err = itErr
		for _, err := range errs {
			if err != nil {
				it.err = errors.Join(it.err, err)
			}
		}
		missingLanguage := mergeValidErrorsMap(validErrors)
		if len(missingLanguage) != 0 {
			logrus.Debugf("Missing language parser for the following file extensions: %#v",
				missingLanguage)
		}
	})
	return it, nil
}

func mergeValidErrorsMap(m []map[string]*expectedError) (
	missingLanguage map[string]int,
) {
	missingLanguage = make(map[string]int)
	for _, mm := range m {
		for k, v := range mm {
			if v.missingLanguage != 0 {
				if _, ok := missingLanguage[k]; !ok {
					missingLanguage[k] = 0
				}
				missingLanguage[k] += v.missingLanguage
			}
		}
	}
	return
}

func readFileSymbols(
	ctx context.Context,
	parser *parser,
	uri workspaceapi.URI,
	w workspaceapi.FileSystem,
	filename string, results chan syntaxapi.Result,
	captureNameFilters []string,
) error {
	file, err := w.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open file: %v", err)
	}

	r := bufio.NewReader(file)
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	buf := new(cell.Buffer)
	buf.Init()
	_, _ = buf.ReadFrom(bytes.NewReader(data))

	tree := parser.parser.Parse(data, nil)
	if tree == nil {
		return errors.New("failed to parse data")
	}
	defer tree.Close()

	cur := sitter.NewQueryCursor()
	defer cur.Close()

	root := tree.RootNode()
	captureNames := parser.query.CaptureNames()
	content := data
	matches := cur.Matches(parser.query, root, content)
	var retErr error
	for {
		m, ok := matches.Next()
		if !ok {
			break
		}
		for _, cap := range m.Captures {
			rng := cap.Node.Range()
			from, to, err := convertRangeToCoordinates(buf.RawCells(), rng)
			if err != nil || int(cap.Index) >= len(captureNames) ||
				(len(captureNameFilters) != 0 &&
					!slices.Contains(captureNameFilters, captureNames[cap.Index])) {
				continue
			}
			fileURI := workspaceapi.Join(uri, filename)
			result, err := makeSymbolItem(from, to, fileURI, buf, captureNames[cap.Index])
			if err != nil {
				retErr = errors.Join(retErr, err)
				continue
			}
			select {
			case results <- result:
			case <-ctx.Done():
				return retErr
			}
		}
	}

	return retErr
}

func makeSymbolItem(
	from, to term.Coordinates, filename workspaceapi.URI, buf *cell.Buffer,
	captureName string,
) (syntaxapi.Result, error) {
	cells, _, _ := buf.Select(from, to)
	textToDisplay := term.CellsToString(cells)
	return syntaxapi.Result{
		CaptureName: captureName,
		From:        from,
		To:          to,
		File:        filename,
		Text:        textToDisplay,
	}, nil
}

func readSymbolsWorker(
	ctx context.Context, w workspaceapi.FileSystem, pkg PkgManager,
	uri workspaceapi.URI, query string, results chan syntaxapi.Result,
	files chan string,
	err *error, expectedErrors map[string]*expectedError,
	captureNameFilters []string,
) {
	parsers := make(map[string]*parser)
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-files:
			if !ok {
				return
			}
			langID, lerr := LanguageForFile(path)
			if lerr != nil {
				continue
			}
			parser, ok := parsers[langID]
			if !ok {
				var perr error
				parser, perr = newParser(ctx, langID, pkg, query)
				if perr != nil {
					if errors.Is(perr, errNotInstalled) {
						ext := filepath.Ext(path)
						if _, ok := expectedErrors[ext]; !ok {
							expectedErrors[ext] = &expectedError{}
						}
						expectedErrors[ext].missingLanguage++
						continue
					}
					perr = fmt.Errorf("new parser for language %q: %v", langID, perr)
					*err = errors.Join(*err, perr)
					return
				}
				defer parser.Close()
				parsers[langID] = parser
			}

			readErr := readFileSymbols(ctx, parser, uri, w, path, results, captureNameFilters)
			if readErr != nil {
				*err = errors.Join(*err, readErr)
			}
		}
	}
}

type listSymbolsIterator struct {
	mu          sync.Mutex
	err         error
	ctx         context.Context
	ch          chan syntaxapi.Result
	cancel      func()
	closeWaitCh chan struct{}
}

func (l *listSymbolsIterator) Next(ctx context.Context) (syntaxapi.Result, bool) {
	select {
	case <-ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = errors.Join(l.err, ctx.Err())
		return syntaxapi.Result{}, false
	case <-l.ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = errors.Join(l.err, l.ctx.Err())
		return syntaxapi.Result{}, false
	case path, ok := <-l.ch:
		return path, ok
	}
}

func (l *listSymbolsIterator) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.err == nil {
		return l.ctx.Err()
	}
	// avoid data races onto l.err which is an instance of
	// *multierr.Error by creating a new multierr.Error
	err := errors.Join(nil, l.err)
	if l.ctx.Err() == nil {
		return err
	}
	return errors.Join(err, l.ctx.Err())
}

func (l *listSymbolsIterator) Close() error {
	l.cancel()
	<-l.closeWaitCh
	return nil
}

var (
	errNotInstalled = errors.New("parser not found: language package not installed")
)

type expectedError struct {
	missingLanguage int
}

type parser struct {
	closed bool
	parser *sitter.Parser
	lang   *sitter.Language
	query  *sitter.Query
	lib    uintptr
}

func newParser(
	ctx context.Context, langID string, pkg PkgManager, query string,
) (ret *parser, err error) {
	it, err := pkg.LibDir(ctx, langID)
	if err != nil {
		err = fmt.Errorf("scan language package installation: %w", err)
		return
	}
	files, err := iterator.ToSlice(ctx, it)
	_ = it.Close()
	if err != nil {
		err = fmt.Errorf("list files: %w", err)
		return
	}
	var langfile string
	for _, path := range files {
		switch filepath.Base(path) {
		case ParserFilename:
			langfile = path
		}
	}

	if langfile == "" {
		err = errNotInstalled
		return
	}

	ret = new(parser)
	ret.lib, err = purego.Dlopen(langfile, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		err = fmt.Errorf("dlopen %q: %w", langfile, err)
		return
	}

	parserID := fmt.Sprintf("tree_sitter_%s", langID)

	var lang func() uintptr
	sym, err := purego.Dlsym(ret.lib, parserID)
	if err != nil {
		_ = purego.Dlclose(ret.lib)
		err = fmt.Errorf("load symbol %q: %w", parserID, err)
		return
	}
	purego.RegisterFunc(&lang, sym)

	language := sitter.NewLanguage(unsafe.Pointer(lang()))
	parser := sitter.NewParser()
	err = parser.SetLanguage(language)
	if err != nil {
		_ = purego.Dlclose(ret.lib)
		parser.Close()
		err = fmt.Errorf("set parser language: %v", err)
		return
	}
	ret.lang = language
	ret.parser = parser

	var qerr *sitter.QueryError
	ret.query, qerr = sitter.NewQuery(ret.lang, query)
	if qerr != nil {
		err = qerr
	}
	if err != nil {
		_ = purego.Dlclose(ret.lib)
		parser.Close()
		err = fmt.Errorf("invalid query: %w", err)
		return
	}
	return
}

func (t *parser) Close() (ret error) {
	if t.closed {
		return nil
	}
	t.closed = true
	t.parser.Close()
	t.query.Close()
	ret = purego.Dlclose(t.lib)
	return
}
