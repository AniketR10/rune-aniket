package document

import "unstable.build/go-tui/workspace"

type readFile struct {
	workspace.File
}

func (f readFile) Sync() error {
	return errUnsupported
}

func (f readFile) Truncate(size int64) error {
	return errUnsupported
}

func (f readFile) Write(p []byte) (n int, err error) {
	return 0, errUnsupported
}

func (f readFile) Close() error {
	return nil
}
