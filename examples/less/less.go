package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/handler"
)

var (
	less *handler.Less
)

var wrap = flag.Bool("w", false, "wrap text")

func startCPUProfile() func() {
	f, err := ioutil.TempFile("", "less_cpuprofile")
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}

func writeMemProfile() {
	f, err := ioutil.TempFile("", "less_memprofile")
	if err != nil {
		log.Fatal("could not create memory profile: ", err)
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Fatal("could not write memory profile: ", err)
	}
}

func handleLessEvent(ev handler.LessEvent) {
	switch ev.Type {
	case handler.EOF:
		less.SetMessage("EOF")
	case handler.Search:
		less.SetMessage("search pattern: %s..", ev.Data)
	}
}

func main() {
	go func() {
		// for net/pprof
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	var input io.Reader
	var err error

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if input, err = os.Open(filename); err != nil {
			log.Fatal(err)
		}
		if err = flag.CommandLine.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	} else {
		input = os.Stdin
		flag.Parse()

	}

	config := handler.DefaultLessConfig()
	config.Wrap = *wrap
	config.Handler = handleLessEvent

	// profile initialization
	stopCPUProfile := startCPUProfile()

	cells := cell.RawCells{}
	_, err = cells.ReadFrom(input)
	if err != nil {
		log.Fatal(err)
	}

	less = handler.NewLess().WithConfig(config)
	less.SetReadWriter(&cells)

	stopCPUProfile()

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	writeMemProfile()

	defer fractal.Close()

	if err = fractal.Run(less); err != nil {
		log.Fatal(err)
	}

}
