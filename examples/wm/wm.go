package main

import (
	"log"

	"github.com/ernestrc/fractal"
)

func main() {
	var err error
	var wm *fractal.WindowManager

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	width, height := fractal.Size()

	wm, _, err = fractal.NewWindowManager(fractal.NewTestHandler(), width, height, 0, 0)
	if err != nil {
		log.Fatal(err)
	}

	if _, err = wm.SplitHorizontal(fractal.NewTestHandler()); err != nil {
		log.Fatal(err)
	}

	if _, err = wm.SplitVertical(fractal.NewTestHandler()); err != nil {
		log.Fatal(err)
	}

	wm.FocusDown()

	if _, err = wm.SplitVertical(fractal.NewTestHandler()); err != nil {
		log.Fatal(err)
	}

	if err := fractal.Run(wm); err != nil {
		log.Fatal(err)
	}
}
