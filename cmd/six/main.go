package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"

	"github.com/ernestrc/go-tui/workspace"
	log "github.com/sirupsen/logrus"
)

var (
	Tag     = "development"
	Commit  = "HEAD"
	Version string

	configpath *string

	flagRecover         = flag.String("r", "", "recover from recovery file")
	flagPprof           = flag.Bool("p", false, "start pprof server at :6060")
	flagVersion         = flag.Bool("v", false, "print version information")
	flagWorkspace       = flag.String("w", cwdURI().String(), "workspace URI")
	flagWorkspaceServer = flag.String("x", "", "runs workspace server from standard input and output")
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	defaultConfigPath := path.Join(home, ".six.yml")
	configpath = flag.String("c", defaultConfigPath, "config file path")

	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func cwdURI() workspace.URI {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %s", err)
	}
	uri, err := workspace.CurrentUserHostURI(wd)
	if err != nil {
		log.Fatalf("Failed to parse working directory as URI %s: %s", wd, err)
	}
	return uri
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

	if *flagWorkspaceServer != "" {
		uri, err := workspace.CurrentUserHostURI(*flagWorkspaceServer)
		if err != nil {
			log.Fatal(err)
		}
		manager, err := workspace.NewManager(uri)
		if err != nil {
			log.Fatal(err)
		}
		server := workspace.NewServer(manager)
		err = workspace.StartWorkspaceServer(server)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	var i *IDE
	if *flagRecover != "" && len(filenames) != 0 {
		i, err = NewRecovery(*flagWorkspace, *configpath, filenames[0], *flagRecover)
	} else if *flagRecover != "" {
		log.Fatal("flag -r requires to pass the original filename filename")
	} else {
		i, err = New(*flagWorkspace, *configpath, filenames...)
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
