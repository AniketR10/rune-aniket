package main

import (
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cmd/plugin_fuzzy_file/finder"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

var (
	defaultCommand           = `set -o pipefail; command find -L . -mindepth 1 \( -path '*/\.*' -o -fstype 'sysfs' -o -fstype 'devfs' -o -fstype 'devtmpfs' -o -fstype 'proc' \) -prune -o -type f -print -o -type l -print 2> /dev/null | cut -b3-`
	defaultHistoryKey        = term.KeyComb{Key: term.KeyCtrlP}
	defaultHistoryDocumentID = "plugin-fuzzy-file-history"
)

func newHandler(grants []plugin.Grant, broker proto.MuxBroker,
	invokeWindow browser.Window, c config.Config) (tui.Handler, error) {
	cmdStr, err := c.GetString("command")
	if err != nil {
		if err != config.ErrNotFound {
			log.Printf("failed to load 'command' config: %v", err)
		}
		cmdStr = defaultCommand
	}
	historyKey, err := config.GetKey(c, "history_key")
	if err != nil {
		if err != config.ErrNotFound {
			log.Printf("failed to load 'command' config: %v", err)
		}
		historyKey = defaultHistoryKey
	}
	return finder.New(grants, broker, invokeWindow, c,
		historyKey, defaultHistoryDocumentID, cmdStr, func(workspace workspace.API, file string) (
			workspace.URI, term.Coordinates,
		) {
			uri, _ := workspace.URI(file)
			return uri, term.Coordinates{}
		})
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	plugutil.ServeCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browser.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          "searchFile",
	})
}
