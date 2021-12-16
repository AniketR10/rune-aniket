package main

import (
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cmd/plugin_fuzzy_file/finder"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

const defaultCommand = `grep -n -r "" .`

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6064", nil))
	}()

	key := term.Event{Type: term.EventKey, Key: term.KeyCtrlBackslash}
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
			return finder.New(grants, broker, invokeWindow,
				config, key, cmdStr, func(data string) (string, term.Coordinates) {
					// NOTE: if ag breaks this or there's an edge case that it's not covered
					// let it panic so we catch it early and fix it
					chunks := strings.Split(data, ":")
					y, _ := strconv.Atoi(chunks[1])
					return chunks[0], term.Coordinates{Y: y - 1}
				})
		},
		Key:         key,
		Permissions: finder.Permissions(),
	})
}
