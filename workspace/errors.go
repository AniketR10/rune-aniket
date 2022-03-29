package workspace

import "errors"

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
