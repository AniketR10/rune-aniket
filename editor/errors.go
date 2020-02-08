package editor

import "errors"

var (
	// ErrFileIsNotRegular is returned when file type was not expected to be a directory.
	ErrFileIsNotRegular = errors.New("file type is not regular")

	// ErrFileAlreadyOpen is returned when a file is not expected to be opened already.
	ErrFileAlreadyOpen = errors.New("file is already open by another application")

	// ErrStaleData is returned when a file was modified by some other application.
	ErrStaleData = errors.New("file was modified by another process since reading it")
)
