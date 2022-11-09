package document

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ernestrc/blue/retry"
	"unstable.build/go-tui/workspace"
)

type fileInfo struct {
	Filename  string
	DataSize  int64
	FileMode  fs.FileMode
	UpdatedAt time.Time
}

type file struct {
	DocID     string
	Data      []byte
	Filename  string
	FileMode  fs.FileMode
	UpdatedAt time.Time

	dirty         bool
	retryStrategy retry.Strategy
	svc           *service
	memFile       workspace.File
}

// return not fully initialzed until init is called
func newFilePrototype(
	docID, filename string, mode fs.FileMode,
) (f *file) {
	ret := new(file)
	ret.DocID = docID
	ret.Filename = filename
	ret.FileMode = mode
	ret.UpdatedAt = time.Now()
	return ret
}

func (f *file) init(svc *service, retryStrategy retry.Strategy) {
	f.svc = svc
	f.memFile = workspace.NewMemoryFile(f.Filename, f.FileMode, f.Data)
	f.retryStrategy = retryStrategy
}

func (f *file) Sync() error {
	f.svc.transaction.Lock()
	defer f.svc.transaction.Unlock()
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, serviceTimeout)
	defer cancel()
	return f.sync(ctx)
}

func (f *file) sync(ctx context.Context) error {
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
	f.Data = buf.Bytes()
	f.UpdatedAt = time.Now()

	return retry.Retry(ctx, f.retryStrategy, func(ctx context.Context) (bool, error) {
		err = f.svc.svc.Set(context.Background(), f.DocID, f)
		return true, err
	})
}

func (f *file) Name() string {
	return f.Filename
}

func (f *file) Stat() (os.FileInfo, error) {
	return fileInfo{
		DataSize:  int64(len(f.Data)),
		Filename:  filepath.Base(f.Filename),
		FileMode:  f.FileMode,
		UpdatedAt: f.UpdatedAt,
	}, nil
}

func (f fileInfo) Name() string {
	return f.Filename
}

func (f fileInfo) Size() int64 {
	return f.DataSize
}

func (f fileInfo) Mode() fs.FileMode {
	return f.FileMode
}

func (f fileInfo) ModTime() time.Time {
	return f.UpdatedAt
}

func (f fileInfo) IsDir() bool {
	return false
}

func (f fileInfo) Sys() any {
	return nil
}

func (f *file) Truncate(size int64) error {
	f.dirty = true
	return f.memFile.Truncate(size)
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return f.memFile.Seek(offset, whence)
}

func (f *file) Read(p []byte) (n int, err error) {
	return f.memFile.Read(p)
}

func (f *file) Write(p []byte) (n int, err error) {
	f.dirty = true
	return f.memFile.Write(p)
}

func (f *file) Close() error {
	if f.dirty {
		return f.Sync()
	}
	return nil
}
