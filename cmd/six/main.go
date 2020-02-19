package main

import (
	"flag"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/editor/vi"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/term"
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

	opts := make([]editor.Option, 0)
	viOpts := make([]vi.Option, 0)

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		opts = append(opts, editor.WithFilepath(filename))
	} else {
		flag.Parse()
	}

	opts = append(opts,
		editor.WithTabspaces(*tabspaces),
		editor.WithSwapDir(*swapDir),
		editor.WithRecoveryFile(*recoveryFile),
	)

	viOpts = append(viOpts,
		vi.WithResAttr(term.Attributes{Bg: term.ColorYellow, Fg: term.ColorBlack}),
	)

	if *debugLog != "" {
		f, err := os.OpenFile(*debugLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}

		l := log.New()
		l.SetOutput(f)
		l.SetLevel(log.TraceLevel)
		opts = append(opts, editor.WithLogger(l))

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
		viOpts = append(viOpts, vi.WithClipboard(clip))
	}

	vi := vi.Editor(viOpts...)
	editor, err := editor.New(vi, opts...)
	if err != nil {
		log.Fatal(err)
	}

	// TODO export EditorHandler and return from New
	if closer, ok := editor.(io.Closer); ok {
		defer closer.Close()
	}

	err = tui.Init()
	if err != nil {
		log.Fatal(err)
	}

	defer tui.Close()

	term.SetOutputMode(term.Output256)

	if err := tui.RunMode(editor, term.InputMouse); err != nil {
		log.Fatal(err)
	}
}
