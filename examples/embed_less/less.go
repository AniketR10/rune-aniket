package main

// import (
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"syscall"
// 	"termbox"
//
// 	"github.com/ernestrc/fractal"
// 	"github.com/ernestrc/fractal/handler"
// )
//
// var (
// 	width, height int
// 	w             fractal.TermboxWriter
// 	e             handler.Embed
// )
//
// func main() {
// 	var err error
//
// 	if len(os.Args) == 1 {
// 		fmt.Printf("usage: %s <filename>", os.Args[0])
// 		os.Exit(1)
// 	}
//
// 	filename := os.Args[1]
// 	c := exec.Command("less", filename)
//
// 	if err = termbox.Init(); err != nil {
// 		log.Fatal(err)
// 	}
//
// 	defer termbox.Close()
//
// 	width, height = termbox.Size()
//
// 	if err = e.Init(0, 0, width, height, c); err != nil {
// 		log.Fatal(err)
// 	}
//
// 	// TODO if err = fractal.Run(&e); err != nil {
// 	// TODO 	log.Fatal(err)
// 	// TODO }
//
// 	data := make([]byte, 1024*4)
//
// 	for {
// 		if c.ProcessState != nil && c.ProcessState.Exited() {
// 			log.Println(c.ProcessState.String())
// 			os.Exit(c.ProcessState.Sys().(*syscall.WaitStatus).ExitStatus())
// 		}
// 		data = data[:cap(data)]
// 		if err = e.Draw(&w); err != nil {
// 			log.Fatal(err)
// 		}
// 		if err = termbox.Flush(); err != nil {
// 			log.Fatal(err)
// 		}
// 		ev := termbox.PollRawEvent(data)
// 		data = data[:ev.N]
// 		if err = e.HandleRaw(data); err != nil {
// 			log.Fatal(err)
// 		}
// 	}
// }
func main() {

}
