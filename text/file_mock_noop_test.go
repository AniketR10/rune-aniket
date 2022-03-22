package text

import "os"

type noopFile struct {
	name string
}

func (f *noopFile) Name() string {
	return f.name
}
func (f *noopFile) Stat() (os.FileInfo, error) {
	return testFileInfo{}, nil
}
func (f *noopFile) Sync() error {
	return nil
}
func (f *noopFile) Truncate(size int64) error {
	return nil
}
func (f *noopFile) WriteString(str string) (int, error) {
	return len(str), nil
}

func (f *noopFile) Seek(offset int64, whence int) (int64, error) {
	return offset, nil
}

func (f *noopFile) Read(p []byte) (n int, err error) {
	return len(p), nil
}

func (f *noopFile) Close() error {
	return nil
}

func (f *noopFile) Write(p []byte) (n int, err error) {
	return len(p), nil
}
