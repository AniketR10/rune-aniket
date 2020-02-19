package main

import (
	"log"

	"github.com/atotto/clipboard"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/plugin"
)

type systemClipboard struct{}

func (c *systemClipboard) Get() (editor.Paste, error) {
	text, err := clipboard.ReadAll()
	if err != nil {
		return editor.Paste{}, err
	}
	return editor.Paste{Data: text}, nil
}

func (c *systemClipboard) Set(data editor.Paste) error {
	return clipboard.WriteAll(data.Data)
}

func main() {
	clip := systemClipboard{}

	if clipboard.Unsupported {
		log.Println("clipboard package does not support this system")
		return
	}

	plugin.ServeClipboard(&clip)
}
