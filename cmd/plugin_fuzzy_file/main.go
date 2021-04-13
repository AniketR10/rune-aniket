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

var (
	defaultCommand = `set -o pipefail; command find -L . -mindepth 1 \( -path '*/\.*' -o -fstype 'sysfs' -o -fstype 'devfs' -o -fstype 'devtmpfs' -o -fstype 'proc' \) -prune -o -type f -print -o -type l -print 2> /dev/null | cut -b3-`
	defaultKey     = term.Event{Type: term.EventKey, Key: term.KeyCtrlP}
)

func newHandler(grants []plugin.Grant, broker proto.MuxBroker,
	invokeWindow browser.Window, config plugin.Config) (tui.Handler, error) {
	cmdStr, err := config.GetString("command")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Printf("failed to load 'command' config: %v", err)
		}
		cmdStr = defaultCommand
	}
	return finder.New(grants, broker, invokeWindow, config,
		defaultKey, cmdStr, func(file string) (string, term.Coordinates) {
			return file, term.Coordinates{}
		})
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	plugutil.ServeKeySplitHandler(plugutil.KeySplitHandlerConfig{
		SplitOrientation: browser.OrientationBottom,
		Handler:          newHandler,
		Key:              defaultKey,
		Permissions:      finder.Permissions(),
		Command:          "searchFile",
	})
}
