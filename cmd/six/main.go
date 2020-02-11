package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/handler"
	"github.com/ernestrc/fractal/plugin"
	"github.com/ernestrc/fractal/term"
	log "github.com/sirupsen/logrus"
)

var debugLog = flag.String("d", "", "debug log file")
var swapDir = flag.String("s", "", "swap files directory")
var recoveryFile = flag.String("r", "", "recover from recovery file")
var clipboardPlugin = flag.String("x", "", "clipboard plugin")
var tabspaces = flag.Int("t", 4, "tabspaces")

func init() {
	log.SetLevel(log.TraceLevel)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	opts := make([]handler.ViOption, 0)

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		opts = append(opts, handler.WithViFilepath(filename))
	} else {
		flag.Parse()
		buf := cell.NewBuffer()
		_, err := buf.ReadFrom(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		opts = append(opts, handler.WithViBuffer(buf))
	}

	opts = append(opts,
		handler.WithViTabspaces(*tabspaces),
		handler.WithViSwapDir(*swapDir),
		handler.WithViRecoveryFile(*recoveryFile),
		handler.WithViResAttr(term.Attributes{Bg: term.ColorYellow, Fg: term.ColorBlack}),
	)

	if *debugLog != "" {
		f, err := os.OpenFile(*debugLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}

		l := log.New()
		l.SetOutput(f)
		l.SetLevel(log.TraceLevel)
		opts = append(opts, handler.WithViLogger(l))

		// set output of plugins
		plugin.SetLoggingOutput(f)
		plugin.SetLoggingLevel(log.TraceLevel)
	}

	if *clipboardPlugin != "" {
		clip, closeClip, err := plugin.NewClipboard(*clipboardPlugin)
		if err != nil {
			log.Fatal(err)
		}
		defer closeClip()
		opts = append(opts, handler.WithViClipboard(clip))
	}

	vi, err := handler.NewVi(opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer vi.Close()

	if err = fractal.Init(); err != nil {
		log.Fatal(err)
	}

	defer fractal.Close()

	if err = fractal.Run(vi); err != nil {
		log.Fatal(err)
	}
}
