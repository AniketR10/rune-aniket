//go:build !windows
// +build !windows

package pty

import (
	"syscall"
	"unsafe"

	workspaceapi "unstable.build/go-tui/api/workspace"
)

// Winsize describes the terminal size.
type Winsize struct {
	Rows uint16 // ws_row: Number of rows (in cells)
	Cols uint16 // ws_col: Number of columns (in cells)
	X    uint16 // ws_xpixel: Width in pixels
	Y    uint16 // ws_ypixel: Height in pixels
}

// Setsize resizes t to s.
func Setsize(t workspaceapi.File, ws *Winsize) error {
	//nolint:gosec // Expected unsafe pointer for Syscall call.
	return ioctl(t.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(ws)))
}

// GetsizeFull returns the full terminal size description.
func GetsizeFull(t workspaceapi.File) (size *Winsize, err error) {
	var ws Winsize

	//nolint:gosec // Expected unsafe pointer for Syscall call.
	if err := ioctl(t.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws))); err != nil {
		return nil, err
	}
	return &ws, nil
}
