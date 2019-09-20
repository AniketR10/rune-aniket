package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"termbox"

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
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

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
		less[i].Init()
		less[i].SetContent(content)
	}

	wm = fractal.NewWindowManager(&less[0], border)

	wm.SplitHorizontal(&less[1])
	wm.SplitVertical(&less[2])
	wm.FocusDown()
	wm.SplitVertical(&less[3])

	if err := fractal.RunMode(wm, termbox.InputAlt); err != nil {
		log.Fatal(err)
	}
}
