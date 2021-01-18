package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"

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
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	plugutil.ServeKeySplitHandler(plugutil.KeySplitHandlerConfig{
		Split:   browser.WindowManager.SplitHorizontalBelow,
		Handler: newFuzzyFinderHandler,
		Key:     term.Event{Type: term.EventKey, Key: term.KeyCtrlP},
	})
}
