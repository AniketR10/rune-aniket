package document

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/retry"
	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/storage"
	"unstable.build/go-tui/workspace"
)

// WorkspaceScheme returns a schemeapi.SchemeFunc that returns schemeapi.Scheme
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
func WorkspaceScheme[T storage.Document[T]](
	rootURI workspaceapi.URI, svc document.Service,
	marshaler encoding.Marshaler, errMissingID error,
) schemeapi.SchemeFunc {
	return func(_ context.Context, cfg config.Config, uri workspaceapi.URI) (
		schemeapi.Scheme, error,
	) {
		if !workspaceapi.HasPrefix(uri, rootURI) {
			return nil, fmt.Errorf("invalid uri %q for scheme with root uri %q:"+
				" root does not match", uri, rootURI)
		}
		ret := new(scheme[T])
		ret.init(svc, uri, marshaler)
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

type scheme[T storage.Document[T]] struct {
	unimplementedTerminal
	unimplementedExecutor
	workspace          workspaceapi.URI
	marshaler          encoding.Marshaler
	svc                service
	errMissingID       error
	retryRealFailure   retry.Strategy
	retryInconsistency retry.Strategy
	files              map[uintptr]workspaceapi.File
	fd                 uintptr // next fd
}

func (s *scheme[T]) init(svc document.Service, uri workspaceapi.URI, m encoding.Marshaler) {
	s.svc.svc = svc
	s.marshaler = m
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
	return workspace.WorkspaceURI(s.workspace, path)
}

func (s *scheme[T]) docIDFromPath(path string) (string, string, error) {
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
		s.files[file.Fd()] = file
	}
	return file, err
}

func (s *scheme[T]) open(
	ctx context.Context, path string, flag int, perm os.FileMode,
) (workspaceapi.File, *workspaceapi.Error) {
	docID, path, err := s.docIDFromPath(path)
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
			return err != document.ErrAlreadyExists, err
		})
		if errors.Is(err, document.ErrAlreadyExists) {
			return nil, &workspaceapi.Error{IsExist: true}
		}
		if err != nil {
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
			return true, err
		})
		if err != nil {
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
		return create || err != document.ErrNotFound, err
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
		return nil, workspaceapi.NopError(err)
	}

	s.fd++
	f, err := newFile(docID, s.marshaler, s.errMissingID,
		s.fd, &s.svc, perm, s.retryRealFailure, ret, !trunc, s)
	if err != nil {
		return nil, workspaceapi.NopError(err)
	}
	return f, nil
}

func (s *scheme[T]) NewFile(fd uintptr, path string) workspaceapi.File {
	f, _ := s.files[fd]
	return f
}

func (s *scheme[T]) Remove(path string) error {
	docID, path, err := s.docIDFromPath(path)
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
		return err != document.ErrNotFound, err
	})
	if errors.Is(err, document.ErrNotFound) {
		return workspaceapi.Error{IsNotExist: true}.ToError()
	}
	if err != nil {
		return err
	}

	return s.retriedDelete(ctx, docID)
}

func (s *scheme[T]) Rename(old, new string) error {
	newDocID, new, err := s.docIDFromPath(new)
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
		return true, err
	})
	if err != nil {
		return err
	}

	cleanup := func(err error) error {
		// create a new context in case we failed due to context.Done
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
		defer cancel()
		rerr := s.retriedDelete(ctx, newDocID)
		if rerr != nil {
			err = multierr.Append(err, rerr)
		}
		return err
	}

	err = s.retriedDelete(ctx, oldDocID)
	if err != nil {
		return cleanup(fmt.Errorf("Delete: %v", err))
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

	name = info.Name()

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
		return true, err
	})
	if err != nil {
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
			gerr := s.svc.svc.Get(ctx, docID, &temp)
			if gerr == document.ErrNotFound {
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
		if gerr == document.ErrNotFound {
			return false, nil
		}
		return true, err
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
