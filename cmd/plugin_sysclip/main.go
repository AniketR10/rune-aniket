package main

import (
	"log"

	"github.com/atotto/clipboard"
	"github.com/ernestrc/fractal/plugin"
)

type systemClipboard struct{}

func (c *systemClipboard) Get() (string, error) {
	return clipboard.ReadAll()
}

func (c *systemClipboard) Set(text string) error {
	return clipboard.WriteAll(text)
}

func main() {
	clip := systemClipboard{}

	if clipboard.Unsupported {
		log.Println("clipboard package does not support this system")
		return
	}

	plugin.ServeClipboard(&clip)
}
