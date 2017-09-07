package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"log"
	"os"

	"github.com/ernestrc/fractal"
)

var (
	vi *fractal.Vi
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
			log.Fatal(err)
		}
		if err = flag.CommandLine.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	} else {
		input = os.Stdin
		flag.Parse()

	}

	h := NewHandler(input)

	initContent := new(bytes.Buffer)
	if _, err = h.WriteTo(initContent); err != nil {
		log.Fatal(err)
	}

	if vi = fractal.NewVi(); err != nil {
		log.Fatal(err)
	}

	if err = vi.SetContent(string(initContent.Bytes())); err != nil {
		log.Fatal(err)
	}

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	if err = fractal.Run(vi); err != nil {
		log.Fatal(err)
	}

}
