package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6062", nil))
	}()

	plugutil.ServeKeySplitHandler(plugutil.KeySplitHandlerConfig{
		Split: browser.WindowManager.SplitVerticalRight,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browser.Window, config plugin.Config) (tui.Handler, error) {
			return new(colorPaletteHandler), nil
		},
		Key: term.Event{Type: term.EventKey, Key: term.KeyCtrlY},
	})
}
