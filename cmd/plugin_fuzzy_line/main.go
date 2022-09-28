package main

import (
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"

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

const (
	defaultCommand           = `grep -n -r "" .`
	defaultHistoryDocumentID = "plugin-fuzzy-line-history"
)

var defaultHistoryKey = term.KeyComb{Key: term.KeyCtrlBackslash}

func parseLine(workspace workspace.API, data string) (
	workspace.URI, term.Coordinates,
) {
	// NOTE: if ag breaks this or there's an edge case that it's not covered
	// let it panic so we catch it early and fix it
	chunks := strings.Split(data, ":")
	y, _ := strconv.Atoi(chunks[1])
	name := chunks[0]
	uri, _ := workspace.URI(name)
	return uri, term.Coordinates{Y: y - 1}
}

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
	return finder.New(grants, broker, invokeWindow,
		c, historyKey, defaultHistoryDocumentID, cmdStr, parseLine)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6064", nil))
	}()

	plugutil.ServeCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browser.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          "searchLine",
	})
}
