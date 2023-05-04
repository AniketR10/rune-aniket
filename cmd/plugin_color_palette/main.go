package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6062", nil))
	}()

	grantee, perms := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browserapi.Window, config config.Config) (browserapi.Handler, error) {
			return new(colorPaletteHandler), nil
		},
		Command: "colorPalette",
	})
	process.Serve(grantee, perms...)
}
