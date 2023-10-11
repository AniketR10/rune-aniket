package extension

import (
	"context"
	_ "net/http/pprof"

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
	// defaultCommand           = `set -o pipefail; command find -L . -mindepth 1 \( -path '*/\.*' -o -fstype 'sysfs' -o -fstype 'devfs' -o -fstype 'devtmpfs' -o -fstype 'proc' \) -prune -o -type f -print -o -type l -print 2> /dev/null | cut -b3-`
	defaultHistoryDocumentID = "extension-fuzzy-file-history"
)

var (
	defaultHistoryKey = term.KeyComb{Key: term.KeyCtrlP}
	cmdSearchFile     = textapi.CommandManual{
		Name: "searchFile",
		Summary: "Opens a new window to perform a fuzzy search for files in the workspace. " +
			"Results are sorted by match score in descending order. " +
			"Arrow keys and <ctrl-k>/<ctrl-j> scroll up and down and <enter> opens up the selected file in a new tab. " +
			"By default, a built-in implementation is used to scan for files in the workspace, but " +
			"for very large workspaces, a program like ripgrep or the silver searcher " +
			"can be used by adding the corresponding 'command' key in the extension's " +
			"configuration (i.e. command: rg -l \"\"). To scroll back to previous searches, " +
			"<ctrl-p> can be used by default or a 'history_key' can be set in the extension's " +
			"configuration. Ctrl-c can be used to cancel a scan in progress.",
	}
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          cmdSearchFile,
	})
}

type stringerStr string

func (s stringerStr) String() string {
	return string(s)
}

func workspaceListFiles(cwd workspaceapi.FileSystem, ctx context.Context) (
	iterator.Iterator[string], error,
) {
	return workspace.ListFiles(ctx, cwd, ".")
}

func getResource(workspace workspaceapi.FileSystem, file string) (
	workspaceapi.URI, term.Coordinates,
) {
	uri, _ := workspace.URI(file)
	return uri, term.Coordinates{}
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
	return finder.New(context.Background(), grants, broker, invokeWindow, c,
		historyKey, defaultHistoryDocumentID, cmdStr,
		workspaceListFiles, getResource)
}
