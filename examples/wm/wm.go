package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/handler"
	"github.com/ernestrc/fractal/term"
)

const border = true

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	if len(os.Args) < 2 {
		fmt.Printf("usage: %s <filename>\n", os.Args[0])
		return
	}

	filename := os.Args[1]
	input, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}

	if err := fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	var wm *handler.WindowManager
	var less [4]handler.Less

	for i := range less {
		less[i].Init()
		less[i].ReadFrom(input)
	}

	wm = handler.NewWindowManager(&less[0], border)

	wm.SplitHorizontal(&less[1])
	wm.SplitVertical(&less[2])
	wm.FocusDown()
	wm.SplitVertical(&less[3])

	if err := fractal.RunMode(wm, term.InputAlt); err != nil {
		log.Fatal(err)
	}
}
