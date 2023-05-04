package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/plugin_fuzzy_file/finder"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/plugin/process"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

const (
	// defaults now to using native workspace.ListFiles if 'command' not defined in config
	// defaultCommand           = `grep -n -r "" .`
	defaultHistoryDocumentID = "plugin-fuzzy-line-history"
)

var defaultHistoryKey = term.KeyComb{Key: term.KeyCtrlBackslash}

func readFiles(cwd workspaceapi.FileSystem, ctx context.Context) (
	iterator.Iterator[string], error,
) {
	it, err := workspace.ListFiles(ctx, cwd, ".")
	if err != nil {
		return nil, err
	}

	return workspace.ReadLines(ctx, cwd, it)
}

func parseLine(workspace workspaceapi.FileSystem, data string) (
	workspaceapi.URI, term.Coordinates,
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
	invokeWindow browserapi.Window, c config.Config) (browserapi.Handler, error) {
	cmdStr, err := c.GetString("command")
	if err != nil {
		if err != config.ErrNotFound {
			log.Printf("failed to load 'command' config: %v", err)
		}
	}
	historyKey, err := config.GetKey(c, "history_key")
	if err != nil {
		if err != config.ErrNotFound {
			log.Printf("failed to load 'command' config: %v", err)
		}
		historyKey = defaultHistoryKey
	}
	return finder.New(grants, broker, invokeWindow,
		c, historyKey, defaultHistoryDocumentID, cmdStr, readFiles, parseLine)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6064", nil))
	}()

	grantee, perms := plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          "searchLine",
	})
	process.Serve(grantee, perms...)
}
