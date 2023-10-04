package extension

import (
	"context"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/extension_fuzzy_file/finder"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

const (
	// defaults now to using native workspace.ListFiles if 'command' not defined in config
	// defaultCommand           = `grep -n -r "" .`
	defaultHistoryDocumentID = "extension-fuzzy-line-history"
)

var defaultHistoryKey = term.KeyComb{Key: term.KeyCtrlBackslash}

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          "searchText",
	})
}

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

func newHandler(cmd textapi.Command, grants []extension.Grant, broker proto.MuxBroker,
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
	return finder.New(context.Background(), grants, broker, invokeWindow,
		c, historyKey, defaultHistoryDocumentID, cmdStr, readFiles, parseLine)
}
