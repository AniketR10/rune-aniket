package main

import (
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cmd/plugin_fuzzy_file/finder"
	"github.com/ernestrc/go-tui/config"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/workspace"
	log "github.com/sirupsen/logrus"
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
