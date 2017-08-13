package main

import (
	"log"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/handler"
)

func main() {
	var err error
	var wm *handler.WindowManager

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	width, height := fractal.Size()

	wm, _, err = handler.NewWindowManager(handler.NewFillHandler(), width, height, 0, 0)
	if err != nil {
		log.Fatal(err)
	}

	if _, err = wm.SplitHorizontal(handler.NewFillHandler()); err != nil {
		log.Fatal(err)
	}

	if _, err = wm.SplitVertical(handler.NewFillHandler()); err != nil {
		log.Fatal(err)
	}

	wm.FocusDown()

	if _, err = wm.SplitVertical(handler.NewFillHandler()); err != nil {
		log.Fatal(err)
	}

	if err := fractal.Run(wm); err != nil {
		log.Fatal(err)
	}
}
