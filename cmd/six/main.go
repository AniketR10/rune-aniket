package main

import (
	"flag"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/handler"
	"github.com/ernestrc/fractal/plugin"
	log "github.com/sirupsen/logrus"
)

var debugLog = flag.String("d", "", "debug log file")
var clipboardPlugin = flag.String("x", "", "clipboard plugin")
var tabspaces = flag.Int("t", 4, "tabspaces")

func init() {
	log.SetLevel(log.TraceLevel)
}

func main() {
	go func() {
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

	cfg := handler.DefaultViConfig
	if *debugLog != "" {
		f, err := os.OpenFile(*debugLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}
		cfg.Logger = log.New()
		cfg.Logger.SetOutput(f)
		cfg.Logger.SetLevel(log.TraceLevel)

		// set output of plugins
		plugin.SetLoggingOutput(f)
		plugin.SetLoggingLevel(log.TraceLevel)
	}

	cfg.Tabspaces = *tabspaces

	if *clipboardPlugin != "" {
		clip, closeClip, err := plugin.NewClipboard(*clipboardPlugin)
		if err != nil {
			log.Fatal(err)
		}
		defer closeClip()
		cfg.Clipboard = clip
	}

	vi := handler.NewVi().WithConfig(cfg)
	_, err = vi.ReadFrom(input)
	if err != nil {
		log.Fatal(err)
	}

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	if err = fractal.Run(vi); err != nil {
		log.Fatal(err)
	}

}
