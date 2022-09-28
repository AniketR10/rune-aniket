package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6062", nil))
	}()

	plugutil.ServeCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browser.OrientationRight,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browser.Window, config config.Config) (tui.Handler, error) {
			return new(colorPaletteHandler), nil
		},
		Command: "colorPalette",
	})
}
