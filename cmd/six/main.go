package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/editor/vi"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

var debugLog = flag.String("d", "", "debug log file")
var swapDir = flag.String("s", "", "swap files directory")
var recoveryFile = flag.String("r", "", "recover from recovery file")
var granteePlugin = flag.String("x", "", "plugin")
var tabspaces = flag.Int("t", 4, "tabspaces")

func init() {
	log.SetLevel(log.TraceLevel)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	opts := make([]browser.Option, 0)
	viOpts := make([]vi.Option, 0)
	pluginOpts := make([]plugin.Option, 0)

	if len(os.Args) > 1 {
		filename := os.Args[1]
		if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		opts = append(opts, browser.WithFilepath(filename))
	} else {
		flag.Parse()
	}

	opts = append(opts,
		browser.WithTabspaces(*tabspaces),
		browser.WithSwapDir(*swapDir),
		browser.WithRecoveryFile(*recoveryFile),
		browser.WithCommandEvent(term.Event{Type: term.EventKey, Ch: ':'}),
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
		opts = append(opts, browser.WithLogger(l))
		viOpts = append(viOpts, vi.WithLogger(l))
		pluginOpts = append(pluginOpts, plugin.WithLogger(l))
	}

	vi := vi.Editor(viOpts...)
	browser, err := browser.New(vi, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close()

	res := plugin.BrowserResources(browser)
	manager := plugin.NewManager(plugin.GrantAll(res), pluginOpts...)
	defer manager.Close()

	if *granteePlugin != "" {
		err := manager.Run(*granteePlugin, *granteePlugin)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = tui.Init()
	if err != nil {
		log.Fatal(err)
	}

	defer tui.Close()

	term.SetOutputMode(term.Output256)
	term.SetInputMode(term.InputMouse)

	err = tui.RunWithLocker(browser, manager.ResourceLocker())
	if err != nil {
		log.Fatal(err)
	}
}
