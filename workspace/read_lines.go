package workspace

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/unstablebuild/blue/iterator"
	multierr "github.com/ernestrc/go-multierror"
)

// ReadLines takes an iterator of file paths, i.e. return of ListFiles
// and returns an iterator of file lines, encoded
func ReadLines(ctx context.Context, w Directory, paths iterator.Iterator[string]) (
	iterator.Iterator[string], error,
) {
	files := make(chan string)
	lines := make(chan string)

	var wg sync.WaitGroup
	wg.Add(defaultWorkers)
	errors := make([]error, defaultWorkers)
	for i := 0; i < defaultWorkers; i++ {
		go func(err *error) {
			defer wg.Done()
			readFileWorker(ctx, w, lines, files, err)
		}(&errors[i])
	}

	it := &listFilesIterator{ctx: ctx, ch: lines}

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
			files <- file
		}
		close(files)
		wg.Wait()

		it.mu.Lock()
		defer it.mu.Unlock()

		it.err = itErr
		for _, err := range errors {
			if err != nil {
				it.err = multierr.Append(it.err, err)
			}
		}
		close(lines)
	}()
	return it, nil
}

func readFile(w Directory, buffer []byte, file string, lines chan string) error {
	f, werr := w.Open(file, os.O_RDONLY, 0)
	if werr != nil {
		return werr.ToError()
	}
	r := bufio.NewScanner(f)
	r.Buffer(buffer, len(buffer))
	var i int
	for r.Scan() {
		i++
		lines <- fmt.Sprintf("%s:%d:%s", file, i, r.Text())
	}
	return r.Err()
}

func readFileWorker(
	ctx context.Context, w Directory,
	lines chan string, files chan string,
	err *error,
) {
	buffer := make([]byte, bufio.MaxScanTokenSize)
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-files:
			if !ok {
				return
			}
			readErr := readFile(w, buffer, path, lines)
			if readErr != nil {
				*err = multierr.Append(*err, readErr)
			}
		}
	}
}
