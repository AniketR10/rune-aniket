package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/retry"
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
	errMissingID  error
	marshaler     encoding.Marshaler
	retryStrategy retry.Strategy
	svc           *service

	val T

	lastSize int
	dirty    bool
	offset   int64
	memFile  workspace.File
}

// return not fully initialzed until init is called
func newFile[T storage.Document[T]](
	docID string, m encoding.Marshaler, errMissingID error,
	svc *service, mode fs.FileMode, retryStrategy retry.Strategy,
	val T, addTemplate bool,
) (*file[T], error) {
	ret := new(file[T])
	ret.marshaler = m
	ret.svc = svc
	ret.val = val
	if addTemplate {
		data, err := ret.marshaler.Marshal(ret.val)
		if err != nil {
			return nil, fmt.Errorf("Marshal: %v", err)
		}
		ret.memFile = workspace.NewMemoryFile(docID, mode, data)
	} else {
		data := make([]byte, 0)
		ret.memFile = workspace.NewMemoryFile(docID, mode, data)
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
	_, err := f.memFile.Seek(0, 0)
	if err != nil {
		panic(err)
	}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, f.memFile)
	if err != nil {
		panic(err)
	}

	// return read offset to its current offset
	_, err = f.memFile.Seek(f.offset, 0)
	if err != nil {
		panic(err)
	}

	var temp T
	err = f.marshaler.Unmarshal(buf.Bytes(), &temp)
	if err != nil {
		return fmt.Errorf("Unmarshal: %v", err)
	}

	f.val = temp
	f.lastSize = buf.Len()
	if f.val.ID() == "" {
		return f.errMissingID
	}

	// we are holding transaction lock so we should be good to make this check:
	// make sure that file still exists in the database before syncing.
	err = retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		var temp T
		err = f.svc.svc.Get(ctx, f.memFile.Name(), &temp)
		return err != document.ErrNotFound, err
	})
	if err != nil {
		return err
	}

	err = retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		err = f.svc.svc.Set(ctx, f.memFile.Name(), f.val)
		return true, err
	})
	if err != nil {
		return err
	}
	// get new UpdatedAt
	return retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		err = f.svc.svc.Get(ctx, f.memFile.Name(), &f.val)
		if err == nil {
			f.dirty = false
		}
		return true, err
	})
}

func (f *file[T]) Name() string {
	return f.memFile.Name()
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
	// do not set to dirty if Truncate only,
	// as we might try to sync an empty buffer
	return f.memFile.Truncate(size)
}

func (f *file[T]) Seek(offset int64, whence int) (int64, error) {
	f.offset = offset
	return f.memFile.Seek(offset, whence)
}

func (f *file[T]) Read(p []byte) (n int, err error) {
	n, err = f.memFile.Read(p)
	f.offset += int64(n)
	return
}

func (f *file[T]) Write(p []byte) (n int, err error) {
	n, err = f.memFile.Write(p)
	f.offset += int64(n)
	return
}

func (f *file[T]) Close() (err error) {
	return nil
}
