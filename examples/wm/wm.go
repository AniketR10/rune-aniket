package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"unstable.build/go-tui"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
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
		less[i].Init(handler.DefaultLessConfig())
		less[i].Buffer().ReadFrom(input)
	}

	wm = handler.NewWindowManager(&less[0], handler.DefaultWindowManagerConfig())

	wm.SplitHorizontal(wm.Focus(), &less[1])
	wm.FocusUp()
	wm.FocusLeft()
	wm.SplitVertical(wm.Focus(), &less[2])
	wm.SplitVertical(wm.Focus(), &less[3])

	term.SetInputMode(term.InputAlt | term.InputMouse)

	if err := tui.Run(wm); err != nil {
		log.Fatal(err)
	}
}
