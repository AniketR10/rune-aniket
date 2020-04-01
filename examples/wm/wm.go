package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

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

	if err := tui.Init(); err != nil {
		log.Fatal(err)
	}

	defer tui.Close()

	var wm *handler.WindowManager
	var less [4]handler.Less

	for i := range less {
		less[i].Init()
		less[i].ReadFrom(input)
	}

	wm = handler.NewWindowManager(&less[0], handler.DefaultWindowManagerConfig())

	wm.SplitHorizontal(&less[1])
	wm.FocusUp()
	wm.FocusLeft()
	wm.SplitVertical(&less[2])
	wm.SplitVertical(&less[3])

	term.SetInputMode(term.InputAlt | term.InputMouse)

	if err := tui.Run(wm); err != nil {
		log.Fatal(err)
	}
}
