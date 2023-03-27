package test

import (
	"io"
	"os"
	"time"
)

// File satisfies workspaceapi.File
type File struct {
}

func (t File) Name() string {
	return ""
}

func (t File) Fd() uintptr {
	return 0
}

func (t File) Stat() (os.FileInfo, error) {
	return FileInfo{}, nil
}

func (t File) Sync() error {
	return nil
}
func (t File) Truncate(size int64) error {
	return nil
}

func (t File) Seek(x int64, y int) (int64, error) {
	return 0, nil
}

func (t File) Read(b []byte) (int, error) {
	return 0, io.EOF
}

func (t File) Write(b []byte) (int, error) {
	return 0, nil
}

func (t File) Close() error {
	return nil
}

// FileInfo satisfies os.FileInfo.
type FileInfo struct {
	Filename    string
	FileIsDir   bool
	FileModTime time.Time
	FileSize    int64
	FileMode    os.FileMode
}

func (t FileInfo) Name() string {
	return t.Filename
}
func (t FileInfo) Size() int64 {
	return t.FileSize
}

func (t FileInfo) Mode() os.FileMode {
	return t.FileMode
}

func (t FileInfo) ModTime() time.Time {
	return t.FileModTime
}

func (t FileInfo) IsDir() bool {
	return t.FileIsDir
}

func (t FileInfo) Sys() interface{} {
	return nil
}
