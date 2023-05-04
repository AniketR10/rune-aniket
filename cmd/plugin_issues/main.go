package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/cmd/plugin_issues/plugin"
	"unstable.build/go-tui/plugin/process"
)

var (
	// compile-time variable
	Tag = "development"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:4568", nil))
	}()

	grantee, perms := plugin.Grantee(Tag)
	process.Serve(grantee, perms...)
}
