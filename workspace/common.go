package workspace

import (
	"io"
	os "os"
)

// used to abstract os.File
type osFile interface {
	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error

	io.Seeker
	io.Reader
	io.Closer
	io.Writer
}

// used to abstract os.Is(.*) functions
type osError struct {
	error
	isPermission bool
	isExist      bool
	isNotExist   bool
}

func (e *osError) Error() string {
	if e.error != nil {
		return e.error.Error()
	}
	if e.isPermission {
		return "permission denied"
	}
	if e.isNotExist {
		return "file does not exist"
	}
	if e.isExist {
		return "file exists"
	}
	return "<osError:nil>"
}

// used to abstract os.Open, os.Remove, os.Rename, os.Stat, os.Lstat and os.ReadLink
type openFunc func(name string, flag int, perm os.FileMode) (osFile, *osError)
type removeFunc func(name string) error
type renameFunc func(oldpath, newpath string) error
type statFunc func(name string) (os.FileInfo, error)
type readLinkFunc func(name string) (string, error)

func osOpenFileFunc() openFunc {
	return func(path string, flag int, perm os.FileMode) (osFile, *osError) {
		f, err := os.OpenFile(path, flag, perm)
		if err != nil {
			return nil, &osError{
				error:        err,
				isPermission: os.IsPermission(err),
				isExist:      os.IsExist(err),
				isNotExist:   os.IsNotExist(err),
			}
		}
		return f, nil
	}
}

func nopOsError(err error) *osError {
	return &osError{error: err}
}
