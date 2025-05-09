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

package docscheme

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/retry"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/schemeapi"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/localstorage"
	"unstable.build/go-tui/workspace"
)

// Scheme returns a schemeapi.SchemeFunc that returns schemeapi.Scheme
// implementations backed by the given document.Service at the given root URI.
//
// Note that rootURI should uniquely identify svc and it will be used
// as the root URI for all schemeapi.Scheme instantiations.
//
// Also note that it's assumed that the underlying documents stored
// are of type T and that it can be safely encoded/decoded with the given
// encoding.Marshaler. Failure to provide this guarantee will probably result
// in corruption of data. The errMissingID argument is used when flushing the
// unmarshaled document if Document.ID returns an empty string.
// the unmarshaled Document.ID.
//
// Open does not support O_APPEND or O_SYNC flags. Write or Close after Write
// do not sync the contents of a file to permanent storage.
// Sync must be called to send data to permanent storage. This is to allow
// for documents to become illegal temporarily while editing.
func Scheme[T localstorage.Document[T]](
	rootURI workspaceapi.URI, svc document.Service,
	marshaler docmarshal.Marshaler, errMissingID error,
	author string,
) schemeapi.SchemeFunc {
	return func(_ context.Context, cfg config.Config, uri workspaceapi.URI) (
		schemeapi.Scheme, error,
	) {
		if !workspaceapi.HasPrefix(uri, rootURI) {
			return nil, fmt.Errorf("invalid uri %q for scheme with root uri %q:"+
				" root does not match", uri, rootURI)
		}
		ret := new(scheme[T])
		ret.init(svc, uri, marshaler, author)
		return ret, nil
	}
}

const (
	// very long timeout just to make sure we don't hang forever
	// since schemeapi.Scheme methods do not take a context (maybe they should!)
	serviceTimeout = 30 * time.Second
)

type service struct {
	svc document.Service
	// provides transaction-level synchronization
	transaction sync.Mutex
}

type scheme[T localstorage.Document[T]] struct {
	unimplementedTerminal
	unimplementedExecutor
	workspace          workspaceapi.URI
	author             string
	marshaler          docmarshal.Marshaler
	svc                service
	errMissingID       error
	retryRealFailure   retry.Strategy
	retryInconsistency retry.Strategy
	mu                 sync.Mutex
	files              map[uintptr]workspaceapi.File
	fd                 uintptr // next fd
}

func (s *scheme[T]) init(
	svc document.Service, uri workspaceapi.URI, m docmarshal.Marshaler, author string,
) {
	s.svc.svc = svc
	s.marshaler = m
	s.author = author
	s.workspace = uri
	s.retryRealFailure = retry.CombinedStrategy(
		retry.ExponentialStrategy(50*time.Microsecond, 500*time.Millisecond),
		retry.LimitStrategy(30),
	)
	s.retryInconsistency = retry.CombinedStrategy(
		retry.ExponentialStrategy(50*time.Microsecond, 250*time.Millisecond),
		retry.LimitStrategy(100),
	)
	s.files = make(map[uintptr]workspaceapi.File)
}

func (s *scheme[T]) URI(path string) (workspaceapi.URI, error) {
	return workspace.NewWorkspaceURI(s.workspace, path)
}

func (s *scheme[T]) MkdirAll(path string, perm os.FileMode) error {
	// no-op, but provide error if file exists
	finfo, err := s.Stat(path)
	if err == nil && !finfo.IsDir() {
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}
	return nil
}

func (s *scheme[T]) docIDFromPath(path string) (string, string, error) {
	// document must not start with absolute path as that has implications
	// for some document stores
	path = filepath.Clean(path)
	if filepath.IsAbs(path) {
		relPath, err := filepath.Rel("/", path)
		if err != nil {
			return "", "", err
		}
		path = relPath
	}
	// document ID is just the pathname, to guarantee
	// compatibility with services that already have data stored
	return path, path, nil
}

func (s *scheme[T]) Open(path string, flag int, perm os.FileMode) (
	workspaceapi.File, *workspaceapi.Error,
) {
	if flag&os.O_APPEND != 0 || flag&os.O_SYNC != 0 {
		return nil, workspaceapi.NopError(errors.New("unsupported Open flag"))
	}

	rdonly := flag&os.O_RDONLY != 0
	wronly := flag&os.O_WRONLY != 0
	rdwr := flag&os.O_RDWR != 0

	if rdonly && wronly {
		return nil, workspaceapi.NopError(
			errors.New("cannot pass O_RDONLY and O_WRONLY at the same time"))
	}
	if rdonly && rdwr || wronly && rdwr {
		return nil, workspaceapi.NopError(
			errors.New("cannot pass O_RDONLY or O_WRONLY with O_RDWR"))
	}

	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	file, err := s.open(ctx, path, flag, perm)
	if err == nil {
		fd := file.Fd()
		s.mu.Lock()
		s.files[fd] = file
		s.mu.Unlock()
	}
	return file, err
}

func (s *scheme[T]) open(
	ctx context.Context, path string, flag int, perm os.FileMode,
) (workspaceapi.File, *workspaceapi.Error) {
	docID, _, err := s.docIDFromPath(path)
	if err != nil {
		return nil, workspaceapi.NopError(err)
	}

	create := flag&os.O_CREATE != 0
	excl := flag&os.O_EXCL != 0
	trunc := flag&os.O_TRUNC != 0

	var template T
	template = template.WithID(docID)
	if create && excl {
		err := retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
			err = s.svc.svc.Create(ctx, docID, template)
			return !errors.Is(err, document.ErrAlreadyExists) &&
				!errors.Is(err, document.ErrPermissionDenied), err
		})
		if err != nil {
			if errors.Is(err, document.ErrAlreadyExists) {
				return nil, &workspaceapi.Error{IsExist: true}
			}
			if errors.Is(err, document.ErrPermissionDenied) {
				return nil, &workspaceapi.Error{IsPermission: true}
			}
			return nil, workspaceapi.NopError(err)
		}
	} else if create || trunc {
		// invariant: in storage there always needs to be
		// a valid unmarshable T value, even if O_TRUNC is passed.
		// In this last case, we leave the inmemory buffer empty for
		// the client to write to and then sync takes care of ensuring
		// that we don't store an invalid structure.
		err := retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
			err = s.svc.svc.Set(ctx, docID, template)
			return !errors.Is(err, document.ErrPermissionDenied), err
		})
		if err != nil {
			if errors.Is(err, document.ErrPermissionDenied) {
				return nil, &workspaceapi.Error{IsPermission: true}
			}
			return nil, workspaceapi.NopError(err)
		}
	}

	// Load any data that was stored previously into f.
	// Some document.Service implementations are eventually consistent,
	// so other than loading any stored data into f, this also exempts the
	// rest of methods from having to worry about stale reads.
	var ret T
	err = retry.Retry(ctx, s.retryInconsistency, func(ctx context.Context) (bool, error) {
		err = s.svc.svc.Get(ctx, docID, &ret)
		return !errors.Is(err, document.ErrPermissionDenied) &&
			(create || !errors.Is(err, document.ErrNotFound)), err
	})
	if err != nil {
		if create {
			// NOTE that this will delete document even when the intention
			// via create flag was to just create if not present.
			_ = s.retriedDelete(ctx, docID)
		}
		if errors.Is(err, document.ErrNotFound) {
			return nil, &workspaceapi.Error{IsNotExist: true}
		}
		if errors.Is(err, document.ErrPermissionDenied) {
			return nil, &workspaceapi.Error{IsPermission: true}
		}
		return nil, workspaceapi.NopError(err)
	}

	s.fd++
	addTemplate := !trunc
	f, err := newFile(docID, s.marshaler, s.errMissingID,
		s.fd, &s.svc, perm, s.retryRealFailure, ret, addTemplate, s)
	if err != nil {
		return nil, workspaceapi.NopError(err)
	}
	return f, nil
}

func (s *scheme[T]) NewFile(fd uintptr, path string) workspaceapi.File {
	s.mu.Lock()
	f := s.files[fd]
	s.mu.Unlock()
	return f
}

func (s *scheme[T]) Remove(path string) error {
	docID, _, err := s.docIDFromPath(path)
	if err != nil {
		return err
	}

	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	var temp T
	err = retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		err = s.svc.svc.Get(ctx, docID, &temp)
		return !errors.Is(err, document.ErrNotFound) &&
			!errors.Is(err, document.ErrPermissionDenied), err
	})
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
			return workspaceapi.Error{IsNotExist: true}.ToError()
		}
		if errors.Is(err, document.ErrPermissionDenied) {
			return workspaceapi.Error{IsPermission: true}.ToError()
		}
		return err
	}

	return s.retriedDelete(ctx, docID)
}

func (s *scheme[T]) Rename(old, new string) error {
	newDocID, _, err := s.docIDFromPath(new)
	if err != nil {
		return err
	}
	oldDocID, old, err := s.docIDFromPath(old)
	if err != nil {
		return err
	}

	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	oldF, werr := s.open(ctx, old, os.O_RDONLY, 0)
	if werr != nil {
		return werr.ToError()
	}

	f := oldF.(*file[T])
	f.val = f.val.WithID(newDocID)

	err = retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		err = s.svc.svc.Set(ctx, newDocID, f.val)
		return !errors.Is(err, document.ErrPermissionDenied), err
	})
	if err != nil {
		if errors.Is(err, document.ErrPermissionDenied) {
			return workspaceapi.Error{IsPermission: true}.ToError()
		}
		return err
	}

	cleanup := func() {
		// create a new context in case we failed due to context.Done
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
		defer cancel()
		_ = s.retriedDelete(ctx, newDocID)
	}

	err = s.retriedDelete(ctx, oldDocID)
	if err != nil {
		cleanup()
		if errors.Is(err, document.ErrPermissionDenied) {
			return workspaceapi.Error{IsPermission: true}.ToError()
		}
		return err
	}

	return nil
}

func (s *scheme[T]) Stat(path string) (os.FileInfo, error) {
	uri, err := s.URI(path)
	if err != nil {
		return nil, fmt.Errorf("URI: %w", err)
	}

	if uri.Path() == "/" {
		return fileInfo{
			filename: s.workspace.Path(),
			isDir:    true,
		}, nil
	}

	if uri.Equal(s.workspace) {
		return fileInfo{
			filename: s.workspace.Path(),
			isDir:    true,
		}, nil
	}

	f, werr := s.Open(path, 0, 0)
	if werr != nil {
		return nil, werr.ToError()
	}
	return f.Stat()
}

func (s *scheme[T]) ReadDir(name string) (
	[]os.DirEntry, error,
) {
	info, err := s.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("not a directory")
	}

	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	ctx := context.Background()

	var it document.Iterator
	err = retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		// name is ignored because even if we knew about the field used
		// as id, there's no "starts with" operation in document.Service
		// and more generally, a document.Service backed Scheme doesn't
		// have any directories.
		it, err = s.svc.svc.List(ctx, nil)
		return !errors.Is(err, document.ErrPermissionDenied), err
	})
	if err != nil {
		if errors.Is(err, document.ErrPermissionDenied) {
			return nil, workspaceapi.Error{IsPermission: true}.ToError()
		}
		return nil, err
	}

	defer it.Close()

	var ret []os.DirEntry
	for it.HasNext() {
		var t T
		err := it.NextTo(&t)
		if err != nil {
			return nil, err
		}
		ret = append(ret, dirEntry{
			name: t.ID(),
		})
	}
	return ret, nil
}

func (s *scheme[T]) Lstat(path string) (os.FileInfo, error) {
	return s.Stat(path)
}

func (s *scheme[T]) ReadLink(path string) (string, error) {
	return "", errors.New("path is not a link")
}

func (s *scheme[T]) Close() error {
	return nil
}

func (s *scheme[T]) retriedDelete(ctx context.Context, docID string) error {
	var temp T
	err := retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		err := s.svc.svc.Delete(ctx, docID)
		if err != nil {
			if errors.Is(err, document.ErrPermissionDenied) {
				return false, workspaceapi.Error{IsPermission: true}.ToError()
			}
			gerr := s.svc.svc.Get(ctx, docID, &temp)
			if errors.Is(gerr, document.ErrNotFound) {
				return false, nil
			}
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	// wait for eventually consistent backends to propagate changes
	return retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		gerr := s.svc.svc.Get(ctx, docID, &temp)
		if errors.Is(gerr, document.ErrNotFound) {
			return false, nil
		}
		return !errors.Is(gerr, document.ErrPermissionDenied), gerr
	})
}

type dirEntry struct {
	name string
}

func (d dirEntry) Name() string {
	return d.name
}

func (d dirEntry) IsDir() bool {
	return false
}

func (d dirEntry) Type() os.FileMode {
	return 0
}

func (d dirEntry) Info() (os.FileInfo, error) {
	panic("unimplemented")
}
