package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/cmd/plugin_logs/plugin"
	"unstable.build/go-tui/plugin/process"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6068", nil))
	}()

	grantee, permissions := plugin.Grantee()
	process.Serve(grantee, permissions...)
}
