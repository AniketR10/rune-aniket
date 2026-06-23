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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/idelsp/languages"
	"unstable.build/go-tui/ide/idelsp/symbolresolve"
	"unstable.build/go-tui/workspace/walkdir"
)

// SearchMulti runs every query in queries against a single parse of each
// workspace file and streams results tagged with the originating query ID.
// Reading, buffering and parsing each file happens once regardless of how
// many queries are supplied, so resolution that previously issued one
// workspace walk per query now issues one walk total. The optional langs
// restrict the walk to files of those languages.
func (p parserSearcher) SearchMulti(
	queries []symbolresolve.MultiQuery, langs ...string,
) (iterator.Iterator[symbolresolve.MultiResult], error) {
	ctx, cancel := context.WithCancel(context.Background())
	ctx = walkdir.WithContextFilter(ctx, p.filter.get())
	paths, err := walkdir.ListFiles(ctx, p.w, ".")
	if err != nil {
		cancel()
		return nil, err
	}

	files := make(chan string)
	results := make(chan symbolresolve.MultiResult)
	closeWaitCh := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(defaultWorkers)
	errs := make([]error, defaultWorkers)
	validErrors := make([]map[string]*expectedError, defaultWorkers)
	for i := range defaultWorkers {
		validErrors[i] = make(map[string]*expectedError)
		err := &errs[i]
		expectedErrors := validErrors[i]
		go debug.CapturePanicReport(func() {
			defer wg.Done()
			readSymbolsWorkerMulti(ctx, p.w, p.pkg, p.uri, queries, results, files,
				err, expectedErrors, langs)
		})
	}

	it := &multiResultIterator{
		ctx:         ctx,
		ch:          results,
		cancel:      cancel,
		closeWaitCh: closeWaitCh,
	}

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
	})
	return it, nil
}

// compiledQueries pairs a loaded language with the per-query compiled
// tree-sitter queries that a worker runs against each file of that language.
type compiledQueries struct {
	lang    *loadedLanguage
	queries []compiledQuery
}

type compiledQuery struct {
	id       int
	query    *sitter.Query
	captures []string
}

func (c *compiledQueries) close() {
	for _, q := range c.queries {
		if q.query != nil {
			q.query.Close()
		}
	}
	if c.lang != nil {
		c.lang.close()
	}
}

func readSymbolsWorkerMulti(
	ctx context.Context, fs workspaceapi.FileSystem, pkg PkgManager,
	uri workspaceapi.URI, queries []symbolresolve.MultiQuery,
	results chan symbolresolve.MultiResult, files chan string,
	err *error, expectedErrors map[string]*expectedError, langs []string,
) {
	compiled := make(map[string]*compiledQueries)
	defer func() {
		for _, c := range compiled {
			c.close()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-files:
			if !ok {
				return
			}
			langID, lerr := languages.LanguageForFile(path)
			if lerr != nil {
				continue
			}
			if len(langs) != 0 && !slices.Contains(langs, langID) {
				continue
			}
			c, ok := compiled[langID]
			if !ok {
				var cerr error
				c, cerr = compileQueriesForLang(ctx, pkg, langID, queries)
				if cerr != nil {
					if errors.Is(cerr, errNotInstalled) {
						ext := filepath.Ext(path)
						if _, ok := expectedErrors[ext]; !ok {
							expectedErrors[ext] = &expectedError{}
						}
						expectedErrors[ext].missingLanguage++
						continue
					}
					*err = errors.Join(*err, cerr)
					continue
				}
				compiled[langID] = c
			}
			if readErr := readFileSymbolsMulti(ctx, c, uri, fs, path, results); readErr != nil {
				*err = errors.Join(*err, readErr)
			}
		}
	}
}

// compileQueriesForLang loads langID once and compiles every query against
// it, so a worker shares a single parser and language handle across all
// queries for that language.
func compileQueriesForLang(
	ctx context.Context, pkg PkgManager, langID string,
	queries []symbolresolve.MultiQuery,
) (*compiledQueries, error) {
	lang, _, err := loadLanguage(ctx, langID, pkg, "", "")
	if err != nil {
		return nil, err
	}
	c := &compiledQueries{lang: lang, queries: make([]compiledQuery, 0, len(queries))}
	for _, q := range queries {
		compiledQ, qerr := compileQuery(lang.lang, q.Query)
		if qerr != nil {
			c.close()
			return nil, fmt.Errorf("new parser for language %q: %v", langID, qerr)
		}
		c.queries = append(c.queries, compiledQuery{
			id: q.ID, query: compiledQ, captures: q.Captures,
		})
	}
	return c, nil
}

func readFileSymbolsMulti(
	ctx context.Context, c *compiledQueries, uri workspaceapi.URI,
	w workspaceapi.FileSystem, filename string,
	results chan symbolresolve.MultiResult,
) (retErr error) {
	file, err := w.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open file: %v", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close file: %v", cerr))
		}
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	tree := c.lang.parser.Parse(content, nil)
	if tree == nil {
		return errors.New("failed to parse data")
	}
	defer tree.Close()
	root := tree.RootNode()
	starts := lineStarts(content)
	fileURI := workspaceapi.Join(uri, filename)

	for _, q := range c.queries {
		if err := runQueryOnTree(ctx, q, root, content, starts, fileURI, results); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}
	return retErr
}

func runQueryOnTree(
	ctx context.Context, q compiledQuery, root *sitter.Node, content []byte,
	starts []int, fileURI workspaceapi.URI,
	results chan symbolresolve.MultiResult,
) (retErr error) {
	cur := sitter.NewQueryCursor()
	defer cur.Close()

	captureNames := q.query.CaptureNames()
	matches := cur.Matches(q.query, root, content)
	for {
		m, ok := matches.Next()
		if !ok {
			break
		}
		for _, cap := range m.Captures {
			if int(cap.Index) >= len(captureNames) ||
				(len(q.captures) != 0 &&
					!slices.Contains(q.captures, captureNames[cap.Index])) {
				continue
			}
			result := makeSymbolItem(
				content, starts, cap.Node.Range(), fileURI, captureNames[cap.Index],
			)
			select {
			case results <- symbolresolve.MultiResult{QueryID: q.id, Result: result}:
			case <-ctx.Done():
				return retErr
			}
		}
	}
	return retErr
}

type multiResultIterator struct {
	mu          sync.Mutex
	err         error
	ctx         context.Context
	ch          chan symbolresolve.MultiResult
	cancel      func()
	closeWaitCh chan struct{}
}

func (l *multiResultIterator) Next(ctx context.Context) (symbolresolve.MultiResult, bool) {
	select {
	case <-ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = errors.Join(l.err, ctx.Err())
		return symbolresolve.MultiResult{}, false
	case <-l.ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = errors.Join(l.err, l.ctx.Err())
		return symbolresolve.MultiResult{}, false
	case r, ok := <-l.ch:
		return r, ok
	}
}

func (l *multiResultIterator) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err == nil {
		return l.ctx.Err()
	}
	err := errors.Join(nil, l.err)
	if l.ctx.Err() == nil {
		return err
	}
	return errors.Join(err, l.ctx.Err())
}

func (l *multiResultIterator) Close() error {
	l.cancel()
	<-l.closeWaitCh
	return nil
}
