package plugin

import (
	"context"
	_ "net/http/pprof"

	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/plugin_fuzzy_file/finder"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

var (
	// defaults now to using native workspace.ListFiles if 'command' not defined in config
	// defaultCommand           = `set -o pipefail; command find -L . -mindepth 1 \( -path '*/\.*' -o -fstype 'sysfs' -o -fstype 'devfs' -o -fstype 'devtmpfs' -o -fstype 'proc' \) -prune -o -type f -print -o -type l -print 2> /dev/null | cut -b3-`
	defaultHistoryKey        = term.KeyComb{Key: term.KeyCtrlP}
	defaultHistoryDocumentID = "plugin-fuzzy-file-history"
)

// Grantee returns this plugin's Grantee and the permissions required to run it.
func Grantee() (plugin.Grantee, []plugin.Permission) {
	return plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationBottom,
		Handler:          newHandler,
		Permissions:      finder.Permissions(),
		Command:          "searchFile",
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
	return finder.New(grants, broker, invokeWindow, c,
		historyKey, defaultHistoryDocumentID, cmdStr,
		workspaceListFiles, getResource)
}
