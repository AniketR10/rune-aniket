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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/debug"
)

// Reader abstracts the ability to read directory contents.
type Reader interface {
	URI(string) (workspaceapi.URI, error)
	OpenFile(path string, flag int, perm os.FileMode) (workspaceapi.File, error)
	Stat(path string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
}

// ListFiles traverses the workspace directory and returns
// an iterator that returns all file paths under root. If root is
// a partial or full file name, it will be ignored and its base
// directory, will be used. If errors are encountered while reading
// the contents of directories those errors will be aggregated and
// reported by the iterator's Err method.
func ListFiles(
	ctx context.Context, w Reader, root string,
) (iterator.Iterator[string], error) {
	return listPaths(ctx, w, root, false)
}

func listPaths(
	ctx context.Context, w Reader, root string, dirOnly bool,
) (iterator.Iterator[string], error) {
	workers := workerCountFromContext(ctx)
	filter := filterFromContext(ctx)
	var wg sync.WaitGroup
	iterCh := make(chan string)
	workerCh := make(chan string)
	closeWaitCh := make(chan struct{})
	allErrors := make([]error, workers)

	workspaceURI, err := w.URI(".")
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}
	rootURI, err := w.URI(root)
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}

	// get root as relative path to workspace
	root = workspaceapi.RelPath(workspaceURI, rootURI)

	iterator := &listFilesIterator{dataCh: iterCh}
	ctx, cancel := context.WithCancel(ctx)
	iterator.ctx = ctx
	iterator.cancel = cancel
	iterator.closeWaitCh = closeWaitCh

	for i := range workers {
		go debug.CapturePanicReport(func() {
			traverseDirWorker(ctx, w, &wg, iterCh, workerCh,
				workspaceURI.Path(), &iterator.mu, &allErrors[i], dirOnly, filter)

		})
	}

	go debug.CapturePanicReport(func() {

		defer close(closeWaitCh)
		defer close(iterCh)
		defer close(workerCh)

		// Root resolution stats the workspace, which on remote
		// schemes is an RPC that can block indefinitely; it must
		// not run on the caller's goroutine (often the UI event
		// loop via completion) — the iterator is what blocks.
		root, rootErr := resolveRootDir(ctx, w, root)
		if rootErr == nil {
			wg.Add(1)
			select {
			case workerCh <- root:
			case <-ctx.Done():
				wg.Done()
				rootErr = ctx.Err()
			}
		}

		wg.Wait()
		iterator.mu.Lock()
		defer iterator.mu.Unlock()
		if rootErr != nil {
			iterator.err = multierr.Append(iterator.err, rootErr)
		}
		for _, err := range allErrors {
			if err != nil {
				iterator.err = multierr.Append(iterator.err, err)
			}
		}

	})

	return iterator, nil
}

// resolveRootDir walks root up to its closest existing directory. Stat
// runs on its own goroutine so that a wedged remote scheme cannot pin
// the traversal teardown: on cancellation the resolution is abandoned
// and the iterator's Close returns promptly.
func resolveRootDir(ctx context.Context, w Reader, root string) (string, error) {
	type result struct {
		root string
		err  error
	}
	resCh := make(chan result, 1)
	go debug.CapturePanicReport(func() {
		for {
			finfo, err := w.Stat(root)
			if err == nil && finfo.IsDir() {
				resCh <- result{root: root}
				return
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				resCh <- result{err: err}
				return
			}
			// Dir is a fixpoint at the filesystem root, so walking past
			// it would spin this goroutine forever when nothing exists
			// (e.g. the scheme was closed under a teardown race). Stop
			// there, and honor cancellation so an abandoned resolution
			// does not keep polling a wedged scheme.
			parent := filepath.Dir(root)
			if parent == root || ctx.Err() != nil {
				if err == nil {
					err = os.ErrNotExist
				}
				resCh <- result{err: err}
				return
			}
			root = parent
		}
	})
	select {
	case res := <-resCh:
		return res.root, res.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// defaultWorkers caps how many goroutines traverse the tree concurrently.
// Directory traversal is syscall-bound (getdents/openat/stat), not CPU-bound:
// past a modest number of workers the kernel's filesystem locks contend and
// throughput collapses. Benchmarking ListFiles over a 3.7M-file tree showed a
// clean U-curve — fastest around NumCPU/2, then steadily worse, with the old
// NumCPU*8 default running ~3x slower than the sweet spot. NumCPU/2 (floored
// at 2 so small machines still parallelize) scales with the host while staying
// left of the collapse. See cmd/walkbench for the measurement.
var defaultWorkers = max(runtime.NumCPU()/2, 2)

func traverseDirWorker(
	ctx context.Context, w Reader, wg *sync.WaitGroup,
	iterCh, workerCh chan string, cwd string, mu *sync.Mutex, err *error,
	dirOnly bool, filter Filter,
) {
	for {
		// do not use ctx here, as we could endup with an outstanding
		// counter on the wait group, which would leak a goroutine.
		path, ok := <-workerCh
		if !ok {
			return
		}
		dirErr := dirTraversal(ctx, w, cwd, path, wg, iterCh, workerCh, dirOnly, filter)
		if dirErr != nil {
			mu.Lock()
			*err = multierr.Append(*err, dirErr)
			mu.Unlock()
		}
	}
}

func dirTraversal(
	ctx context.Context, w Reader, cwd, dirname string,
	wg *sync.WaitGroup, iterCh, workerCh chan string,
	dirOnly bool, filter Filter,
) error {
	defer wg.Done()
	absPath := dirname
	if !filepath.IsAbs(dirname) {
		absPath = filepath.Join(cwd, dirname)
	}

	dirNames, err := w.ReadDir(absPath)
	if err != nil {
		return err
	}

	var ret error
	for _, info := range dirNames {
		path := filepath.Join(dirname, info.Name())
		tpe := info.Type()
		// path is workspace-relative (ListFiles normalizes root via
		// workspaceapi.RelPath before recursing), so we can match the
		// filter without an extra w.URI() round trip per entry.
		if filter != nil && filter.MatchRelPath(path, tpe.IsDir()) {
			continue
		}
		if tpe.IsRegular() && !dirOnly {
			// ensure dirTraversal returns
			select {
			case <-ctx.Done():
				return ctx.Err()
			case iterCh <- path:
				continue
			}
		}
		if !tpe.IsDir() {
			continue
		}

		// stream directories
		if dirOnly {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case iterCh <- path:
			}
		}

		wg.Add(1)
		select {
		// ensure dirTraversal returns
		case <-ctx.Done():
			wg.Done()
			return ctx.Err()
		case workerCh <- path:
		default:
			// the rest of workers are busy, keep going
			err := dirTraversal(ctx, w, cwd, path, wg, iterCh, workerCh, dirOnly, filter)
			if err != nil {
				ret = multierr.Append(ret, err)
			}
		}
	}
	return ret
}

type listFilesIterator struct {
	mu          sync.Mutex
	err         error
	ctx         context.Context
	dataCh      chan string
	cancel      func()
	closeWaitCh chan struct{}
}

func (l *listFilesIterator) Next(ctx context.Context) (string, bool) {
	select {
	case <-ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = multierr.Append(l.err, ctx.Err())
		return "", false
	case <-l.ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = multierr.Append(l.err, l.ctx.Err())
		return "", false
	case path, ok := <-l.dataCh:
		return path, ok
	}
}

func (l *listFilesIterator) Close() error {
	l.cancel()
	<-l.closeWaitCh
	return nil
}

func (l *listFilesIterator) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.err == nil {
		return l.ctx.Err()
	}
	// avoid data races onto l.err which is an instance of
	// *multierr.Error by creating a new multierr.Error
	err := multierr.Append(nil, l.err)
	if l.ctx.Err() == nil {
		return err
	}
	return multierr.Append(err, l.ctx.Err())
}
