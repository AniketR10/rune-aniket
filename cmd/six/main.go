package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
	"unstable.build/go-tui/workspace/ssh"
)

var (
	// compile-time variables
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
	defaultConfigPath := path.Join(home, ".sixrc")
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

	proto.DisableGRPCLogging()

	if *flagWorkspaceServer != "" {
		uri, err := workspace.CurrentUserHostURI(*flagWorkspaceServer)
		if err != nil {
			log.Fatal(err)
		}
		l := log.New()
		l.Out = ioutil.Discard
		l.Level = log.PanicLevel
		manager, err := workspace.NewFileScheme(config.NopConfig(), uri)
		if err != nil {
			log.Fatal(err)
		}
		server := workspacepb.NewSchemeServer(manager)
		err = ssh.StartSchemeServer(server)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	var i *ide
	if *flagRecover != "" && len(filenames) != 0 {
		i, err = newIdeRecovery(*flagWorkspace, *configpath, filenames[0], *flagRecover)
	} else if *flagRecover != "" {
		log.Fatal("flag -r requires to pass the original filename filename")
	} else {
		i, err = newIde(*flagWorkspace, *configpath, filenames...)
	}

	if err != nil {
		log.Fatal(err)
	}

	err = i.run()
	i.Close()
	if err != nil {
		log.Fatal(err)
	}
}
