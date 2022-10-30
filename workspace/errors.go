package workspace

import (
	"errors"
	"os"
)

var (
	// ErrFileIsNotRegular is returned when file type was not expected to be a directory.
	ErrFileIsNotRegular = errors.New("file type is not regular")

	// ErrFileIsNotWritable is returned when file is opened in read-only.
	ErrFileIsNotWritable = errors.New("file is not writable")

	// ErrFileAlreadyOpen is returned when a file is not expected to be opened already.
	ErrFileAlreadyOpen = errors.New("file open by another process or previous process was closed abruptly")

	// ErrStaleData is returned when a file was modified by some other application.
	ErrStaleData = errors.New("file was modified by another process since reading it")
)

// Error is used to abstract os.Is(.*) functions
type Error struct {
	Err          error
	IsPermission bool
	IsExist      bool
	IsNotExist   bool
}

func (e *Error) String() string {
	if e == nil {
		return "<nil>"
	}

	return e.ToError().Error()
}

func (e Error) ToError() error {
	if e.IsPermission {
		return os.ErrPermission
	}
	if e.IsNotExist {
		return os.ErrNotExist
	}
	if e.IsExist {
		return os.ErrExist
	}
	if e.Err != nil {
		return e.Err
	}
	panic("workspace.Error with nil Error")
}

// NopError returns an Error that simply wraps err.
func NopError(err error) *Error {
	return &Error{Err: err}
}
