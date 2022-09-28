package main

import (
	"log"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/text/vi"
	"unstable.build/go-tui/workspace"
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

	uri, err := workspace.ParseURI("vi:///test")
	if err != nil {
		log.Fatal(err)
	}

	editor := handler.NewFrame(vi.New(b, uri))

	err = tui.Run(editor)
	if err != nil {
		log.Fatal(err)
	}
}
