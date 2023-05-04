package main

import (
	_ "net/http/pprof"

	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
)

func main() {
	grantee, perms := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browserapi.Window, config config.Config) (browserapi.Handler, error) {
			return new(panicHandler), nil
		},
		Command: "panicPlugin",
	})
	process.Serve(grantee, perms...)
}
