package plugin

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	sitter "github.com/smacker/go-tree-sitter"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/cmd/plugin_lexer/plugin"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

var (
	defaultWorkers     int
	errUnknownLanguage = errors.New("unknown language")
	errInvalidQuery    = errors.New("invalid query for language")
)

func init() {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	defaultWorkers = int(math.Min(float64(maxProcs), float64(numCPU)))
}

func readSymbols(
	ctx context.Context, w workspace.Directory,
	paths iterator.Iterator[string], query string,
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
			readSymbolsWorker(ctx, w, query, results, files, err, missingLanguage)
		}(&errors[i], validErrors[i])
	}

	it := &listSymbolsIterator{ctx: ctx, ch: results}

	var itErr error
	go func() {
		for {
			file, ok := paths.Next()
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
	w workspace.Directory, query string,
	filename string, results chan string,
) error {
	parser, lang, ok := plugin.NewParser(filename)
	if !ok {
		return errUnknownLanguage
	}

	defer parser.Close()

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
	// FIXME pass tabspaces
	buf.InitWithTabspaces(cell.DefaultTabspaces)
	buf.Write(data)

	tree, err := parser.ParseCtx(ctx, nil, data)
	if err != nil {
		return err
	}

	q, err := sitter.NewQuery([]byte(query), lang)
	if err != nil {
		return errInvalidQuery
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	var retErr error
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
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
	from, ok := cell.ConvertRuneCoordinates(c, int(start.Row), int(start.Column))
	if !ok {
		err = fmt.Errorf("convert points: failed to convert sitter 'start point "+
			" to term 'from' coordinates: point: %v", start)
		return
	}
	to, ok = cell.ConvertRuneCoordinates(c, int(end.Row), int(end.Column))
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
	cells, _ := buf.Select(from, to)
	textToDisplay := cell.CellsToString(cells)
	return fmt.Sprintf("%s:%d: %s", filename, from.Y+1, textToDisplay), nil
}

func readSymbolsWorker(
	ctx context.Context, w workspace.Directory,
	query string, functions chan string, files chan string,
	err *error, missingLanguage map[string]*commonErrors,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-files:
			if !ok {
				return
			}
			readErr := readFileSymbols(ctx, w, query, path, functions)
			if readErr != nil {
				if readErr == errUnknownLanguage {
					ext := filepath.Ext(path)
					if _, ok := missingLanguage[ext]; !ok {
						missingLanguage[ext] = &commonErrors{}
					}
					missingLanguage[ext].missingLanguage++
				} else if readErr == errInvalidQuery {
					ext := filepath.Ext(path)
					if _, ok := missingLanguage[ext]; !ok {
						missingLanguage[ext] = &commonErrors{}
					}
					missingLanguage[ext].invalidQuery++
				} else {
					*err = multierror.Append(*err, readErr)
				}
			}
		}
	}
}

type listSymbolsIterator struct {
	mu  sync.Mutex
	err error
	ctx context.Context
	ch  chan string
}

func (l *listSymbolsIterator) Next() (string, bool) {
	select {
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

type commonErrors struct {
	missingLanguage int
	invalidQuery    int
}
