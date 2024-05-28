package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/retry"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/storage"
	"unstable.build/go-tui/workspace"
)

type fileInfo struct {
	filename string
	dataSize int64
	fileMode fs.FileMode
	modified time.Time
	isDir    bool
}

type file[T storage.Document[T]] struct {
	s             *scheme[T]
	errMissingID  error
	marshaler     encoding.Marshaler
	retryStrategy retry.Strategy
	svc           *service

	val T

	lastSize int
	memFile  workspaceapi.File
}

// return not fully initialzed until init is called
func newFile[T storage.Document[T]](
	docID string, m encoding.Marshaler, errMissingID error,
	fd uintptr, svc *service, mode fs.FileMode,
	retryStrategy retry.Strategy,
	val T, addTemplate bool, s *scheme[T],
) (*file[T], error) {
	ret := new(file[T])
	ret.marshaler = m
	ret.svc = svc
	ret.val = val
	ret.s = s
	if addTemplate {
		data, err := ret.marshaler.Marshal(ret.val)
		if err != nil {
			return nil, fmt.Errorf("Marshal: %v", err)
		}
		ret.memFile = workspace.NewMemoryFile(docID, fd, mode, data, &s.mu)
	} else {
		data := make([]byte, 0)
		ret.memFile = workspace.NewMemoryFile(docID, fd, mode, data, &s.mu)
	}
	ret.retryStrategy = retryStrategy
	return ret, nil
}

func (f *file[T]) Sync() error {
	f.svc.transaction.Lock()
	defer f.svc.transaction.Unlock()
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	return f.sync(ctx)
}

func (f *file[T]) sync(ctx context.Context) error {
	// NOTE: we're potentially losing read/write
	// offset position by doing this
	if _, err := f.memFile.Seek(0, 0); err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	_, err := io.Copy(&buf, f.memFile)
	if err != nil {
		panic(err)
	}

	var temp T
	err = f.marshaler.Unmarshal(buf.Bytes(), &temp)
	if err != nil {
		return fmt.Errorf("Unmarshal: %v", err)
	}

	f.val = temp
	f.val = f.val.WithUpdatedTime(time.Now())
	if f.val.ID() == "" {
		return f.errMissingID
	}
	f.val = f.val.WithUpdatedBy(f.s.author)

	// we are holding transaction lock so we should be good to make this check:
	// make sure that file still exists in the database before syncing.
	err = retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		var temp T
		err = f.svc.svc.Get(ctx, f.memFile.Name(), &temp)
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

	err = retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		err = f.svc.svc.Set(ctx, f.memFile.Name(), f.val)
		return !errors.Is(err, document.ErrPermissionDenied), err
	})
	if err != nil {
		if errors.Is(err, document.ErrPermissionDenied) {
			return workspaceapi.Error{IsPermission: true}.ToError()
		}
		return err
	}

	// ensure what's in the database is exaclty what we have loaded in f.val and
	// stored in memFile as data (some document.Service impls automatically updtate
	// created at, updated at fields, auto-incremented IDs, etc.
	err = retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		err = f.svc.svc.Get(ctx, f.memFile.Name(), &f.val)
		return !errors.Is(err, document.ErrPermissionDenied), err
	})
	if err != nil {
		if errors.Is(err, document.ErrPermissionDenied) {
			return workspaceapi.Error{IsPermission: true}.ToError()
		}
		return err
	}

	// write updated time back into memory file
	data, err := f.marshaler.Marshal(f.val)
	if err != nil {
		return fmt.Errorf("Unmarshal: %v", err)
	}

	if err := f.memFile.Truncate(0); err != nil {
		panic(err)
	}

	if _, err := f.memFile.Write(data); err != nil {
		panic(err)
	}

	f.lastSize = len(data)

	return nil
}

func (f *file[T]) Name() string {
	return f.memFile.Name()
}

func (f *file[T]) Fd() uintptr {
	return f.memFile.Fd()
}

func (f *file[T]) Stat() (os.FileInfo, error) {
	mstat, _ := f.memFile.Stat()
	return fileInfo{
		dataSize: int64(f.lastSize),
		// there shouldn't be any directories so it's safe to just return orig name
		filename: f.val.ID(),
		fileMode: mstat.Mode(),
		modified: f.val.UpdatedTime(),
	}, nil
}

func (f fileInfo) Name() string {
	return f.filename
}

func (f fileInfo) Size() int64 {
	return f.dataSize
}

func (f fileInfo) Mode() fs.FileMode {
	return f.fileMode
}

func (f fileInfo) ModTime() time.Time {
	return f.modified
}

func (f fileInfo) IsDir() bool {
	return f.isDir
}

func (f fileInfo) Sys() any {
	return nil
}

func (f *file[T]) Truncate(size int64) error {
	return f.memFile.Truncate(size)
}

func (f *file[T]) Seek(offset int64, whence int) (int64, error) {
	return f.memFile.Seek(offset, whence)
}

func (f *file[T]) Read(p []byte) (n int, err error) {
	return f.memFile.Read(p)
}

func (f *file[T]) Write(p []byte) (n int, err error) {
	return f.memFile.Write(p)
}

func (f *file[T]) Close() (err error) {
	delete(f.s.files, f.Fd())
	return nil
}
