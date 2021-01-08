package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
)

var configpath *string
var recfilename = flag.String("r", "", "recover from recovery file")
var pprof = flag.Bool("p", false, "start pprof server at :6060")

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

	for _, file := range flag.Args() {
		filenames = append(filenames, file)
	}

	if *pprof {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		go func() {
			log.Println(http.ListenAndServe(":6060", nil))
		}()
	}

	var i *IDE
	if *recfilename != "" && len(filenames) != 0 {
		i, err = NewRecovery(*configpath, filenames[0], *recfilename)
	} else if *recfilename != "" {
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
