// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace/walkdir"
)

var (
	defaultWorkers     = runtime.NumCPU()
	errUnknownLanguage = errors.New("unknown language")
	errInvalidQuery    = errors.New("invalid query for language")
)

func readSymbols(
	ctx context.Context, w walkdir.Reader,
	paths iterator.Iterator[string], query queryType,
	tabspaces int, queryFn func(string) (string, error),
) (iterator.Iterator[string], error) {
	files := make(chan string)
	results := make(chan string)

	var wg sync.WaitGroup
	wg.Add(defaultWorkers)
	errors := make([]error, defaultWorkers)
	validErrors := make([]map[string]*commonErrors, defaultWorkers)
	for i := 0; i < defaultWorkers; i++ {
		validErrors[i] = make(map[string]*commonErrors)
		go func(err *error, missingLanguage map[string]*commonErrors) {
			defer wg.Done()
			readSymbolsWorker(ctx, w, query, results, files, err,
				missingLanguage, tabspaces, queryFn)
		}(&errors[i], validErrors[i])
	}

	it := &listSymbolsIterator{ctx: ctx, ch: results}
	ctx, it.cancel = context.WithCancel(ctx)

	var itErr error
	go func() {
		for {
			file, ok := paths.Next(ctx)
			if !ok {
				if err := paths.Err(); err != nil {
					itErr = err
				}
				break
			}
			select {
			case files <- file:
			case <-ctx.Done():
				return
			}
		}
		close(files)
		wg.Wait()

		it.mu.Lock()
		defer it.mu.Unlock()

		it.err = itErr
		for _, err := range errors {
			if err != nil {
				it.err = multierror.Append(it.err, err)
			}
		}
		missingLanguage, invalidQuery := mergeValidErrorsMap(validErrors)
		if len(missingLanguage) != 0 {
			log.Debugf("Missing language parser for the following file extensions: %#v",
				missingLanguage)
		}
		if len(invalidQuery) != 0 {
			log.Debugf("Invalid query for the following file extensions: %#v",
				invalidQuery)
		}
		close(results)
	}()
	return it, nil
}

func mergeValidErrorsMap(m []map[string]*commonErrors) (
	missingLanguage map[string]int,
	invalidQuery map[string]int,
) {
	missingLanguage = make(map[string]int)
	invalidQuery = make(map[string]int)
	for _, mm := range m {
		for k, v := range mm {
			if v.missingLanguage != 0 {
				if _, ok := missingLanguage[k]; !ok {
					missingLanguage[k] = 0
				}
				missingLanguage[k] += v.missingLanguage
			}
			if v.invalidQuery != 0 {
				if _, ok := invalidQuery[k]; !ok {
					invalidQuery[k] = 0
				}
				invalidQuery[k] += v.invalidQuery
			}
		}
	}
	return
}

func readFileSymbols(
	ctx context.Context,
	w walkdir.Reader, queryType queryType,
	filename string, results chan string,
	tabspaces int,
	queryFn func(string) (string, error),
) error {
	parser, lang, ok := NewParser(filename)
	if !ok {
		return errUnknownLanguage
	}
	defer parser.Close()

	query, err := queryFn(filename)
	if err != nil {
		return err
	}

	file, werr := w.Open(filename, os.O_RDONLY, 0)
	if werr != nil {
		return werr.ToError()
	}

	r := bufio.NewReader(file)
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	buf := new(cell.Buffer)
	buf.InitWithTabspaces(tabspaces)
	_, _ = buf.Write(data)

	tree, err := parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return err
	}

	q, err := sitter.NewQuery([]byte(query), lang)
	if err != nil {
		return errInvalidQuery
	}
	defer q.Close()

	qc := sitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	// use FilterPredicates if there are any
	var filterPredicates bool
	for i := uint32(0); i < q.PatternCount(); i++ {
		steps := q.PredicatesForPattern(i)
		if len(steps) == 0 {
			continue
		}
		filterPredicates = true
		break
	}

	var retErr error
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		if filterPredicates {
			m = qc.FilterPredicates(m, data)
		}

		for _, c := range m.Captures {
			if queryType != queryTypeCustom && q.CaptureNameForId(c.Index) != string(queryType) {
				continue
			}

			result, err := makeSymbolItem(filename, buf, c.Node)
			if err != nil {
				retErr = multierror.Append(retErr, err)
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

func convertStartEndPoints(
	n *sitter.Node, buf *cell.Buffer,
) (from, to term.Coordinates, err error) {
	c := buf.RawCells()
	start, end := n.StartPoint(), n.EndPoint()
	from, ok := cell.ConvertRunePosToCoordinates(c, int(start.Row), int(start.Column))
	if !ok {
		err = fmt.Errorf("convert points: failed to convert sitter 'start point "+
			" to term 'from' coordinates: point: %v", start)
		return
	}
	to, ok = cell.ConvertRunePosToCoordinates(c, int(end.Row), int(end.Column))
	if !ok {
		err = fmt.Errorf("convert points: failed to convert sitter 'end' point "+
			" to term 'to' coordinates: point: %v", end)
		return
	}
	return
}

func makeSymbolItem(filename string, buf *cell.Buffer, n *sitter.Node) (string, error) {
	from, to, err := convertStartEndPoints(n, buf)
	if err != nil {
		return "", err
	}
	to.Y = from.Y
	to.X = buf.Columns(from.Y)
	cells, _, _ := buf.Select(from, to)
	textToDisplay := cell.CellsToString(cells)
	return fmt.Sprintf("%s:%d: %s", filename, from.Y+1, textToDisplay), nil
}

func readSymbolsWorker(
	ctx context.Context, w walkdir.Reader,
	query queryType, functions chan string, files chan string,
	err *error, missingLanguage map[string]*commonErrors,
	tabspaces int, queryFn func(string) (string, error),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-files:
			if !ok {
				return
			}
			readErr := readFileSymbols(ctx, w, query, path, functions, tabspaces, queryFn)
			if readErr != nil {
				switch readErr {
				case errUnknownLanguage:
					ext := filepath.Ext(path)
					if _, ok := missingLanguage[ext]; !ok {
						missingLanguage[ext] = &commonErrors{}
					}
					missingLanguage[ext].missingLanguage++
				case errInvalidQuery:
					ext := filepath.Ext(path)
					if _, ok := missingLanguage[ext]; !ok {
						missingLanguage[ext] = &commonErrors{}
					}
					missingLanguage[ext].invalidQuery++
				default:
					*err = multierror.Append(*err, readErr)
				}
			}
		}
	}
}

type listSymbolsIterator struct {
	mu     sync.Mutex
	err    error
	ctx    context.Context
	ch     chan string
	cancel func()
}

func (l *listSymbolsIterator) Next(ctx context.Context) (string, bool) {
	select {
	case <-ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = multierror.Append(l.err, ctx.Err())
		return "", false
	case <-l.ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = multierror.Append(l.err, l.ctx.Err())
		return "", false
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
	err := multierror.Append(nil, l.err)
	if l.ctx.Err() == nil {
		return err
	}
	return multierror.Append(err, l.ctx.Err())
}

func (l *listSymbolsIterator) Close() error {
	l.cancel()
	return nil
}

type commonErrors struct {
	missingLanguage int
	invalidQuery    int
}
