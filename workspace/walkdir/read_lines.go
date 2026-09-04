// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package walkdir

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"unstable.build/rune/debug"
)

// ReadLines takes an iterator of file paths, i.e. return of ListFiles
// and returns an iterator of file lines, encoded
func ReadLines(ctx context.Context, w Reader, paths iterator.Iterator[string]) (
	iterator.Iterator[string], error,
) {
	workers := workerCountFromContext(ctx)
	maxTokenSize := max(scanBufferSizeFromContext(ctx), bufio.MaxScanTokenSize)
	files := make(chan string)
	lines := make(chan string)
	closeWaitCh := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup
	wg.Add(workers)
	errors := make([]error, workers)
	for i := range workers {
		err := &errors[i]
		go debug.CapturePanicReport(func() {

			defer wg.Done()
			readFileWorker(ctx, w, maxTokenSize, lines, files, err)

		})
	}

	it := &listFilesIterator{
		ctx:    ctx,
		dataCh: lines,
	}
	it.cancel = cancel
	it.closeWaitCh = closeWaitCh

	var itErr error
	go debug.CapturePanicReport(func() {

		defer close(closeWaitCh)
		defer close(lines)
		defer paths.Close() //nolint:errcheck

	loop:
		for {
			file, ok := paths.Next(ctx)
			if !ok {
				if err := paths.Err(); err != nil {
					itErr = err
				}
				break
			}
			select {
			case <-ctx.Done():
				break loop
			case files <- file:
			}
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

	})
	return it, nil
}

func readFile(
	ctx context.Context, w Reader, buffer []byte, maxTokenSize int,
	file string, lines chan string,
) (retErr error) {
	f, err := w.OpenFile(file, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			retErr = multierr.Append(retErr, err)
		}
	}()
	r := bufio.NewScanner(f)
	r.Buffer(buffer, maxTokenSize)
	var i int
	for r.Scan() {
		i++
		data := r.Bytes()
		if bytes.IndexByte(data, 0) != -1 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case lines <- fmt.Sprintf("%s:%d:%s", file, i, data):
		}
	}
	// A single line larger than the scanner's buffer (e.g. minified JS,
	// generated JSON, log lines) surfaces as bufio.ErrTooLong. Silently stop
	// scanning that file instead of aggregating the error onto the iterator;
	// callers care about errors that affect the search as a whole, not about
	// individual files we cannot tokenize.
	if err := r.Err(); err != nil && !errors.Is(err, bufio.ErrTooLong) {
		return err
	}
	return nil
}

func readFileWorker(
	ctx context.Context, w Reader, maxTokenSize int,
	lines chan string, files chan string,
	err *error,
) {
	// The buffer starts at the scanner default and only lines that
	// need more grow it (per file, transiently): a read's size is the
	// buffer's free space, and over the workspace RPC that size becomes
	// the file server's allocation, so sizing every buffer to a raised
	// cap turns a scan of many small files into a stream of cap-sized
	// server allocations.
	buffer := make([]byte, min(maxTokenSize, bufio.MaxScanTokenSize))
	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-files:
			if !ok {
				return
			}
			readErr := readFile(ctx, w, buffer, maxTokenSize, path, lines)
			if readErr != nil {
				*err = multierr.Append(*err, readErr)
			}
		}
	}
}
