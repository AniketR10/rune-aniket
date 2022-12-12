package api

import "errors"

var (
	// ErrInvalidSave is returned when trying to save a buffer that it's not a file
	// in the file system.
	ErrInvalidSave = errors.New("Cannot save this buffer")

	// ErrInvalidSplit is returned when attempting to split over a floating window.
	ErrInvalidSplit = errors.New("Cannot split this window")
)
