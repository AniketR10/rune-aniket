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

var ag = `ag --nogroup --nocolor '^(?=.)'`

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6064", nil))
	}()

	key := term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash}
	plugutil.ServeKeySplitHandler(plugutil.KeySplitHandlerConfig{
		SplitOrientation: browser.OrientationBottom,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browser.Window, config plugin.Config) (tui.Handler, error) {
			return finder.New(grants, broker, invokeWindow,
				config, key, ag, func(data string) string {
					for i, c := range data {
						if c == ':' {
							return data[:i]
						}
					}
					return ""
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
