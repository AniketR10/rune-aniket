package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
)

var (
	Version = "development"
	configpath *string

	flagRecover = flag.String("r", "", "recover from recovery file")
	flagPprof = flag.Bool("p", false, "start pprof server at :6060")
	flagVersion = flag.Bool("v", false, "print version information")
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	defaultConfigPath := path.Join(home, ".six.yml")
	configpath = flag.String("c", defaultConfigPath, "config file path")
}

func main() {
	var err error
	var filenames []string

	flag.Parse()

	if *flagVersion {
		fmt.Printf("Six %s\n", Version)
		return
	}

	for _, file := range flag.Args() {
		filenames = append(filenames, file)
	}

	if *flagPprof {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		go func() {
			log.Println(http.ListenAndServe(":6060", nil))
		}()
	}

	var i *IDE
	if *flagRecover!= "" && len(filenames) != 0 {
		i, err = NewRecovery(*configpath, filenames[0], *flagRecover)
	} else if *flagRecover != "" {
		log.Fatal("flag -r requires to pass the original filename filename")
	} else {
		i, err = New(*configpath, filenames...)
	}

	if err != nil {
		log.Fatal(err)
	}

	defer i.Close()
	err = i.Run()
	if err != nil {
		log.Fatal(err)
	}
}
