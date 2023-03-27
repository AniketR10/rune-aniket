package workspace

import (
	"os"

	workspaceapi "unstable.build/go-tui/api/workspace"
)

// InvalidFile returns a File that always returns the given err.
// This can be used by Scheme implementations to satisfy NewFile
// when callers passed an invalid file descriptor.
func InvalidFile(fd uintptr, name string, err error) workspaceapi.File {
	return invalidFile{fd: fd, name: name, err: err}
}

type invalidFile struct {
	fd   uintptr
	name string
	err  error
}

func (c invalidFile) Read(p []byte) (n int, err error) {
	return 0, c.err
}

func (c invalidFile) Write(p []byte) (n int, err error) {
	return 0, c.err
}

func (c invalidFile) Name() string {
	return c.name
}

func (c invalidFile) Stat() (os.FileInfo, error) {
	return nil, c.err
}

func (c invalidFile) Sync() error {
	return c.err
}

func (c invalidFile) Truncate(size int64) error {
	return c.err
}

func (c invalidFile) Fd() uintptr {
	return c.fd
}

func (c invalidFile) Seek(offset int64, whence int) (int64, error) {
	return 0, c.err
}

func (c invalidFile) Close() error {
	return c.err
}
