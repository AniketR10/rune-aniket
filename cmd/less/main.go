package main

import (
	"bufio"
	"io"
	"os"

	"github.com/ernestrc/less"
)

type ScannerHandler struct {
	scanner *bufio.Scanner
}

func NewHandler(r io.Reader) *ScannerHandler {
	handler := new(ScannerHandler)
	handler.scanner = bufio.NewScanner(r)
	return handler
}

func (h ScannerHandler) WriteTo(w io.Writer) (written int64, err error) {
	var n int
	for h.scanner.Scan() {
		if err = h.scanner.Err(); err != nil {
			return
		}
		if n, err = w.Write([]byte(h.scanner.Text() + "\n")); err != nil {
			return
		}
		written += int64(n)
	}
	return written, nil
}

func (h ScannerHandler) OnEvent(ev *less.Event) error {
	return nil
}

func main() {
	handler := NewHandler(os.Stdin)
	handle := less.New(handler, nil)

	if err := handle.Run(); err != nil {
		panic(err)
	}
}
