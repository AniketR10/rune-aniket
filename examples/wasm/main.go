package main

import (
	"log"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor/vi"
	"github.com/ernestrc/go-tui/handler"
)

const text = `
                                $$\               $$\       
                                $$ |              \__|      
    $$$$$$\   $$$$$$\         $$$$$$\   $$\   $$\ $$\       
   $$  __$$\ $$  __$$\ $$$$$$\\_$$  _|  $$ |  $$ |$$ |      
   $$ /  $$ |$$ /  $$ |\______| $$ |    $$ |  $$ |$$ |      
   $$ |  $$ |$$ |  $$ |         $$ |$$\ $$ |  $$ |$$ |      
   \$$$$$$$ |\$$$$$$  |         \$$$$  |\$$$$$$  |$$ |      
    \____$$ | \______/           \____/  \______/ \__|      
   $$\   $$ |                                               
   \$$$$$$  |                                               
    \______/`

func main() {
	err := tui.Init()
	if err != nil {
		log.Fatal(err)
	}
	defer tui.Close()

	b := cell.NewBuffer()
	b.WriteString(text)

	editor := handler.NewFrame(vi.New(b))

	err = tui.Run(editor)
	if err != nil {
		log.Fatal(err)
	}
}
