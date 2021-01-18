package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)
	plugin.SetLoggingLevel(log.DebugLevel)

	go func() {
		log.Println(http.ListenAndServe("localhost:6062", nil))
	}()

	plugutil.ServeKeySplitHandler(plugutil.KeySplitHandlerConfig{
		Split: browser.WindowManager.SplitVerticalRight,
		Handler: func(f browser.ResourceOpener, p browser.EventPublisher,
			invokeWindow browser.Window, config plugin.Config,
		) tui.Handler {
			return new(colorPaletteHandler)
		},
		Key: term.Event{Type: term.EventKey, Key: term.KeyCtrlY},
	})
}
