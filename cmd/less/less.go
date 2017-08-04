package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ernestrc/fractal/less"
	termbox "termbox"
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

	return
}

var wrap = flag.Bool("w", false, "wrap text")
var border = flag.Bool("b", false, "window borders")

func main() {

	var input io.Reader
	var err error

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if input, err = os.Open(filename); err != nil {
			panic(err)
		}
		if err = flag.CommandLine.Parse(os.Args[2:]); err != nil {
			panic(err)
		}
	} else {
		input = os.Stdin
		flag.Parse()

	}

	config := less.DefaultConfig()
	config.Wrap = *wrap
	if *border {
		config.ScrollBorder |= termbox.ColorWhite
	}

	handler := NewHandler(input)
	initContent := new(bytes.Buffer)
	if _, err = handler.WriteTo(initContent); err != nil {
		panic(err)
	}

	if err = less.Init(config, string(initContent.Bytes())); err != nil {
		panic(err)
	}

	var i int
	for {
		ev := less.PollEvent()

		switch ev.Type {
		case less.EOF:
			i++
			less.Message("dispatched EOF n %d", i)
		case less.Search:
			less.Message("dispatched search: %s..", ev.Data)
		case less.Error:
			panic(ev.Err)
		case less.Exit:
			less.Close()
			fmt.Println("bye!")
			return
		}
	}
}
