package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ernestrc/fractal"
)

func readFile(f io.Reader) (string, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func main() {
	var err error
	var input io.Reader
	var content string

	if len(os.Args) < 2 {
		fmt.Printf("usage: %s <filename>\n", os.Args[0])
		return
	}

	filename := os.Args[1]
	if input, err = os.Open(filename); err != nil {
		log.Fatal(err)
	}

	if content, err = readFile(input); err != nil {
		log.Fatal(err)
	}

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	var wm *fractal.WindowManager
	var l1, l2, l3, l4 *fractal.Less

	if l1, err = fractal.NewLess(content, nil, nil); err != nil {
		log.Fatal(err)
	}

	if l2, err = fractal.NewLess(content, nil, nil); err != nil {
		log.Fatal(err)
	}

	if l3, err = fractal.NewLess(content, nil, nil); err != nil {
		log.Fatal(err)
	}

	if l4, err = fractal.NewLess(content, nil, nil); err != nil {
		log.Fatal(err)
	}

	wm, _ = fractal.NewWindowManager(l1)

	if _, err = wm.SplitHorizontal(l2); err != nil {
		log.Fatal(err)
	}

	if _, err = wm.SplitVertical(l3); err != nil {
		log.Fatal(err)
	}

	wm.FocusDown()

	if _, err = wm.SplitVertical(l4); err != nil {
		log.Fatal(err)
	}

	if err := fractal.Run(wm); err != nil {
		log.Fatal(err)
	}
}
