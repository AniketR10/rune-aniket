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
			return written, err
		}
		if n, err = w.Write([]byte(h.scanner.Text() + "\n")); err != nil {
			return
		}
		written += int64(n)
	}

	// subsequent calls test
	if written == 0 {
		if n, err = w.Write([]byte("no more data\n")); err != nil {
			return
		}
		written += int64(n)
	}

	return
}

func (h ScannerHandler) OnSearch(l *less.Handle, text string) error {
	l.Message("searching for %s..", text)
	return nil
}

func main() {

	var input io.Reader
	var err error

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if input, err = os.Open(filename); err != nil {
			panic(err)
		}
	} else {
		input = os.Stdin
	}

	handler := NewHandler(input)

	handle := less.New(handler, nil)

	if err := handle.Run(); err != nil {
		panic(err)
	}
}
