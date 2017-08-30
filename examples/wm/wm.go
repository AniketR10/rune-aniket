package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ernestrc/fractal"
)

const border = true

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
	var less [4]fractal.Less

	for i := range less {
		if err = less[i].Init(nil); err != nil {
			log.Fatal(err)
		}

		if err = less[i].SetContent(content); err != nil {
			log.Fatal(err)
		}
	}

	wm, _ = fractal.NewWindowManager(&less[0], border)

	if _, err = wm.SplitHorizontal(&less[1]); err != nil {
		log.Fatal(err)
	}

	if _, err = wm.SplitVertical(&less[2]); err != nil {
		log.Fatal(err)
	}

	wm.FocusDown()

	if _, err = wm.SplitVertical(&less[3]); err != nil {
		log.Fatal(err)
	}

	if err := fractal.Run(wm); err != nil {
		log.Fatal(err)
	}
}
