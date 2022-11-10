package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/retry"
	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

// WorkspaceScheme returns a workspace.SchemeFunc that returns workspace.Scheme
// implementations backed by the given document.Service at the given root URI.
//
// Note that rootURI should uniquely identify svc and it will be used
// as the root URI for all workspace.Scheme instantiations.
//
// Also note that it's assumed that no other process is writing to the resources
// managed by the given document.Service. Failure to provide this guarantee
// will probably result in corruption of data.
//
// Open does not support O_APPEND or O_SYNC flags.
func WorkspaceScheme(rootURI workspace.URI, svc document.Service) workspace.SchemeFunc {
	return func(cfg config.Config, uri workspace.URI) (workspace.Scheme, error) {
		if !workspace.HasPrefix(uri, rootURI) {
			return nil, fmt.Errorf("invalid uri %q for scheme with root uri %q: root does not match", uri, rootURI)
		}
		ret := new(scheme)
		ret.init(svc, uri)
		return ret, nil
	}
}

const (
	// very long timeout just to make sure we don't hang forever
	// since workspace.Scheme methods do not take a context (maybe they should!)
	serviceTimeout = 30 * time.Second
)

type service struct {
	svc document.Service
	// provides transaction-level synchronization
	transaction sync.Mutex
}

type scheme struct {
	unimplementedTerminal
	unimplementedExecutor
	workspace          workspace.URI
	svc                service
	retryRealFailure   retry.Strategy
	retryInconsistency retry.Strategy
}

func (s *scheme) init(svc document.Service, uri workspace.URI) {
	s.svc.svc = svc
	s.workspace = uri
	s.retryRealFailure = retry.CombinedStrategy(
		retry.ExponentialStrategy(100*time.Millisecond, 500*time.Millisecond),
		retry.LimitStrategy(30),
	)
	s.retryInconsistency = retry.CombinedStrategy(
		retry.ExponentialStrategy(50*time.Millisecond, 250*time.Millisecond),
		retry.LimitStrategy(100),
	)
}

func (s *scheme) URI(path string) (workspace.URI, error) {
	return workspace.WorkspaceURI(s.workspace, path)
}

func (s *scheme) docIDFromPath(path string) (string, string, error) {
	path = filepath.Clean(path)
	if path == "" {
		return "", "", fmt.Errorf("invalid file %q", path)
	}
	uri, err := s.URI(path)
	if err != nil {
		return "", "", err
	}
	// document ID is just the pathname, to guarantee
	// compatibility with services that already have data stored
	return uri.Path(), path, nil
}

func (s *scheme) Open(path string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	if flag&os.O_APPEND != 0 || flag&os.O_SYNC != 0 {
		return nil, workspace.NopError(errors.New("unsupported Open flag"))
	}

	rdonly := flag&os.O_RDONLY != 0
	wronly := flag&os.O_WRONLY != 0
	rdwr := flag&os.O_RDWR != 0

	if rdonly && wronly {
		return nil, workspace.NopError(
			errors.New("cannot pass O_RDONLY and O_WRONLY at the same time"))
	}
	if rdonly && rdwr || wronly && rdwr {
		return nil, workspace.NopError(
			errors.New("cannot pass O_RDONLY or O_WRONLY with O_RDWR"))
	}

	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	return s.open(ctx, path, flag, perm)
}

func (s *scheme) open(ctx context.Context, path string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	docID, path, err := s.docIDFromPath(path)
	if err != nil {
		return nil, workspace.NopError(err)
	}

	create := flag&os.O_CREATE != 0
	excl := flag&os.O_EXCL != 0
	trunc := flag&os.O_TRUNC != 0

	var filename string
	rel, err := filepath.Rel(s.workspace.Path(), path)
	if err != nil {
		filename = filepath.Join(s.workspace.Path(), path)
	} else {
		filename = filepath.Join(s.workspace.Path(), rel)
	}

	f := newFilePrototype(docID, filename, perm)
	if create && excl {
		err := retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
			err = s.svc.svc.Create(ctx, docID, f)
			return err != document.ErrAlreadyExists, err
		})
		if errors.Is(err, document.ErrAlreadyExists) {
			return nil, &workspace.Error{IsExist: true}
		}
		if err != nil {
			return nil, workspace.NopError(err)
		}
	} else if create || trunc {
		err := retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
			err = s.svc.svc.Set(ctx, docID, f)
			return true, err
		})
		if err != nil {
			return nil, workspace.NopError(err)
		}
	}

	// Load any data that was stored previously into f.
	// Some document.Service implementations are eventually consistent,
	// so other than loading any stored data into f, this also exempts the
	// rest of methods from having to worry about stale reads.
	err = retry.Retry(ctx, s.retryInconsistency, func(ctx context.Context) (bool, error) {
		err = s.svc.svc.Get(ctx, docID, f)
		return create || err != document.ErrNotFound, err
	})
	if err != nil {
		if create {
			// NOTE that this will delete document even when the intention
			// via create flag was to just create if not present.
			_ = s.retriedDelete(ctx, docID)
		}
		if errors.Is(err, document.ErrNotFound) {
			return nil, &workspace.Error{IsNotExist: true}
		}
		return nil, workspace.NopError(err)
	}

	// initialize File API now that data has been loaded
	f.init(&s.svc, s.retryRealFailure)

	return f, nil
}

func (s *scheme) Remove(path string) error {
	docID, path, err := s.docIDFromPath(path)
	if err != nil {
		return err
	}

	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()

	var temp file
	err = retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		err = s.svc.svc.Get(ctx, docID, &temp)
		return err != document.ErrNotFound, err
	})
	if errors.Is(err, document.ErrNotFound) {
		return workspace.Error{IsNotExist: true}.ToError()
	}
	if err != nil {
		return err
	}

	return s.retriedDelete(ctx, docID)
}

func (s *scheme) Rename(old, new string) error {
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

	oldF, werr := s.open(ctx, old, os.O_RDONLY, 0666)
	if werr != nil {
		return werr.ToError()
	}
	newF, werr := s.open(ctx, new, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if werr != nil {
		return werr.ToError()
	}

	cleanup := func(ctx context.Context, err error) error {
		rerr := s.retriedDelete(ctx, newDocID)
		if rerr != nil {
			err = multierr.Append(err, rerr)
		}
		return err
	}

	_, err = io.Copy(newF, oldF)
	if err != nil {
		return cleanup(ctx, fmt.Errorf("Copy: %v", err))
	}

	err = newF.(*file).sync(ctx)
	if err != nil {
		return cleanup(ctx, fmt.Errorf("Sync: %v", err))
	}

	err = s.retriedDelete(ctx, oldDocID)
	if err != nil {
		return cleanup(ctx, fmt.Errorf("Delete: %v", err))
	}

	return nil
}

func (s *scheme) Stat(path string) (os.FileInfo, error) {
	f, werr := s.Open(path, 0, 0)
	if werr != nil {
		return nil, werr.ToError()
	}
	return f.Stat()
}

func (s *scheme) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	s.svc.transaction.Lock()
	defer s.svc.transaction.Unlock()

	var it document.Iterator
	var err error
	err = retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
		it, err = s.svc.svc.List(ctx, nil)
		return true, err
	})
	if err != nil {
		return nil, err
	}
	return iterator.Map(iterator.FromDocumentIterator[*file](it), func(in *file) string {
		filename, _ := filepath.Rel(s.workspace.Path(), in.Name())
		return filename
	}), nil
}

func (s *scheme) Lstat(path string) (os.FileInfo, error) {
	return s.Stat(path)
}

func (s *scheme) ReadLink(path string) (string, error) {
	return "", errors.New("path is not a link")
}

func (s *scheme) Close() error {
	return nil
}

func (s *scheme) retriedDelete(ctx context.Context, docID string) error {
	var temp file
	return retry.Retry(ctx, s.retryRealFailure, func(ctx context.Context) (bool, error) {
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
}
