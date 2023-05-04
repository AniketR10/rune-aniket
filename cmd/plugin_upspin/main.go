package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/cmd/plugin_upspin/plugin"
	"unstable.build/go-tui/plugin/process"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:4568", nil))
	}()

	grantee, perms := plugin.Grantee()
	process.Serve(grantee, perms...)
}
