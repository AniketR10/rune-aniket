package document

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/iterator"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

// Identifiable abstracts a document that knows about the ID
// used to indexed it in the underlying document.Service.
type Identifiable interface {
	ID() string
}

// WorkspaceSchemeView returns a workspace.SchemeFunc that returns workspace.Scheme
// read-only implementations backed by the given document.Service at the given root URI.
//
// Open does not support O_APPEND, O_SYNC, O_CREATE, O_EXCL, O_TRUNC, O_WRONLY or O_RDWR flags
// so effectively files are only available for read. In exchange, this implementation provides
// a read only workspace.Scheme that can interoperate with an existing document.Service dataset
// as oposed to the workspace.Scheme returned by WorkspaceScheme.
//
// Open returns a File that reads the results of marshaling T into a slice of bytes
// with given marshaler.
func WorkspaceSchemeView[T Identifiable](
	rootURI workspace.URI, svc document.Service, marshaler encoding.Marshaler,
) workspace.SchemeFunc {
	return func(cfg config.Config, uri workspace.URI) (workspace.Scheme, error) {
		if !workspace.HasPrefix(uri, rootURI) {
			return nil, fmt.Errorf("invalid uri %q for scheme with root uri %q: root does not match", uri, rootURI)
		}
		ret := new(readScheme[T])
		ret.init(svc, uri, marshaler)
		return ret, nil
	}
}

var (
	errUnsupported = errors.New("write operations are unsupported")
)

type readScheme[T Identifiable] struct {
	unimplementedTerminal
	unimplementedExecutor
	workspace workspace.URI
	svc       document.Service
	marshaler encoding.Marshaler
}

func (s *readScheme[T]) init(svc document.Service, uri workspace.URI, m encoding.Marshaler) {
	s.svc = svc
	s.workspace = uri
	s.marshaler = m
}

func (s *readScheme[T]) Open(path string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	if flag&os.O_APPEND != 0 || flag&os.O_SYNC != 0 ||
		flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0 ||
		flag&os.O_EXCL != 0 || flag&os.O_CREATE != 0 || flag&os.O_TRUNC != 0 {
		return nil, workspace.NopError(errors.New("unsupported Open flag"))
	}

	var temp T
	ctx := context.Background()
	err := s.svc.Get(ctx, path, &temp)
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
			return nil, &workspace.Error{IsNotExist: true}
		}
		return nil, workspace.NopError(err)
	}

	data, err := s.marshaler.Marshal(temp)
	if err != nil {
		return nil, workspace.NopError(fmt.Errorf("Marshal: %v", err))
	}

	var filename string
	rel, err := filepath.Rel(s.workspace.Path(), path)
	if err != nil {
		filename = filepath.Join(s.workspace.Path(), path)
	} else {
		filename = filepath.Join(s.workspace.Path(), rel)
	}

	return readFile{File: workspace.NewMemoryFile(filename, perm, data)}, nil
}

func (s *readScheme[T]) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	it, err := s.svc.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	return iterator.Map(iterator.FromDocumentIterator[T](it), func(in T) string {
		return in.ID()
	}), nil
}

func (s *readScheme[T]) URI(path string) (workspace.URI, error) {
	return workspace.WorkspaceURI(s.workspace, path)
}

func (s *readScheme[T]) Stat(path string) (os.FileInfo, error) {
	f, werr := s.Open(path, 0, 0)
	if werr != nil {
		return nil, werr.ToError()
	}
	return f.Stat()
}

func (s *readScheme[T]) Lstat(path string) (os.FileInfo, error) {
	return s.Stat(path)
}

func (s *readScheme[T]) ReadLink(path string) (string, error) {
	return "", errors.New("path is not a link")
}

func (s *readScheme[T]) Close() error {
	return nil
}

/* not implemented */

func (s *readScheme[T]) Remove(path string) error {
	return errUnsupported
}

func (s *readScheme[T]) Rename(old, new string) error {
	return errUnsupported
}
