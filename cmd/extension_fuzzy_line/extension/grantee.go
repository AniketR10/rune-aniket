package extension

import (
	"context"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
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

var (
	defaultHistoryKey = term.KeyComb{Key: term.KeyCtrlBackslash}
	cmdSearchText     = textapi.CommandManual{
		Name: "searchText",
		Summary: "Opens a new window to perform a fuzzy search for file contents in the workspace. " +
			"Results are sorted by match score in descending order. " +
			"Arrow keys and <ctrl-k>/<ctrl-j> scroll up and down and <enter> opens up the selected file in a new tab. " +
			"By default, a built-in implementation is used to scan for files in the workspace, but " +
			"for very large workspaces, a program like ripgrep or the silver searcher " +
			"can be used by adding the corresponding 'command' key in the extension's " +
			"configuration or by passing an argument (i.e. searchText rg --color never -n --no-heading --max-columns 500 \"\"" +
			"). To scroll back to previous searches, " +
			`<ctrl-\> can be used by default or a 'history_key' can be set in the extension's ` +
			"configuration. Ctrl-c can be used to cancel a scan in progress.",
		Synopsis: "[command]",
	}
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          cmdSearchText,
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

func newHandler(
	ctx context.Context, cmd textapi.Command,
	grants []extension.Grant, broker proto.MuxBroker,
	invokeWindow browserapi.Window, c config.Config,
) (browserapi.Handler, error) {
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
	if len(cmd.Args) != 0 {
		cmdStr = strings.Join(cmd.Args, " ")
	}
	return finder.New(ctx, grants, broker, invokeWindow,
		c, historyKey, defaultHistoryDocumentID, cmdStr, readFiles, parseLine)
}
