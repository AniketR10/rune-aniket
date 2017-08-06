// +build !windows

package pty

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	ws_row    uint16
	ws_col    uint16
	ws_xpixel uint16
	ws_ypixel uint16
}

// Getsize returns the number of rows (lines) and cols (positions
// in each line) in terminal t.
func Getsize(t *os.File) (rows, cols int, err error) {
	var ws winsize
	err = windowrect(&ws, syscall.TIOCGWINSZ, t.Fd())
	return int(ws.ws_row), int(ws.ws_col), err
}

func Setsize(t *os.File, rows, cols int) error {
	ws := winsize{uint16(rows), uint16(cols), 0, 0}
	return windowrect(&ws, syscall.TIOCSWINSZ, t.Fd())
}

func windowrect(ws *winsize, flag, fd uintptr) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		flag,
		uintptr(unsafe.Pointer(ws)),
	)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}
