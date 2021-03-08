package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cmd/plugin_fuzzy_file/finder"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

var defaultCommand = `set -o pipefail; command find -L . -mindepth 1 \( -path '*/\.*' -o -fstype 'sysfs' -o -fstype 'devfs' -o -fstype 'devtmpfs' -o -fstype 'proc' \) -prune -o -type f -print -o -type l -print 2> /dev/null | cut -b3-`

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	key := term.Event{Type: term.EventKey, Key: term.KeyCtrlP}

	plugutil.ServeKeySplitHandler(plugutil.KeySplitHandlerConfig{
		SplitOrientation: browser.OrientationBottom,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browser.Window, config plugin.Config) (tui.Handler, error) {
			cmdStr, err := config.GetString("command")
			if err != nil {
				if err != plugin.ErrNotFound {
					log.Printf("failed to load 'command' config: %v", err)
				}
				cmdStr = defaultCommand
			}
			return finder.New(grants, broker, invokeWindow, config,
				key, cmdStr, func(file string) string {
					return file
				})
		},
		Key: key,
		Permissions: []plugin.Permission{
			plugin.PermissionBrowserResourceOpener,
			plugin.PermissionBrowserEventPublisher,
			plugin.PermissionBrowserMessenger,
			plugin.PermissionBrowserStorage,
		},
	})
}
