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
	less *fractal.Less
)

type scannerHandler struct {
	scanner *bufio.Scanner
}

func newHandler(r io.Reader) *scannerHandler {
	handler := new(scannerHandler)
	handler.scanner = bufio.NewScanner(r)
	return handler
}

func (h scannerHandler) WriteTo(w io.Writer) (written int64, err error) {
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

func (h scannerHandler) Handle(ev fractal.LessEvent) {
	switch ev.Type {
	case fractal.EOF:
		less.SetMessage("EOF")
	case fractal.Search:
		less.SetMessage("search pattern: %s..", ev.Data)
	}
}

var wrap = flag.Bool("w", false, "wrap text")

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

	config := fractal.DefaultLessConfig()
	config.Wrap = *wrap

	h := newHandler(input)

	config.Handler = h.Handle

	initContent := new(bytes.Buffer)
	if _, err = h.WriteTo(initContent); err != nil {
		log.Fatal(err)
	}

	less = fractal.NewLess()
	less.InitWithConfig(config)

	less.SetContent(string(initContent.Bytes()))

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	if err = fractal.Run(less); err != nil {
		log.Fatal(err)
	}

}
