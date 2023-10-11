//go:build windows
// +build windows

package pty

import (
	"os"

	workspaceapi "unstable.build/go-tui/api/workspace"
)

// Winsize is a dummy struct to enable compilation on unsupported platforms.
type Winsize struct {
	Rows, Cols, X, Y uint16
}

// Setsize resizes t to s.
func Setsize(workspaceapi.File, *Winsize) error {
	return ErrUnsupported
}

// GetsizeFull returns the full terminal size description.
func GetsizeFull(workspaceapi.File) (*Winsize, error) {
	return nil, ErrUnsupported
}
