package util

import (
	"fmt"
	"io"
	"sync"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
)

type cmdSplitHandler struct {
	mu     sync.Mutex
	config CommandSplitHandlerConfig

	broker  proto.MuxBroker
	wm      browser.WindowManager
	ed      editor.Editor
	pconfig plugin.Config
	grants  []plugin.Grant
	h       tui.Handler
	win     browser.Window
}

func (t *cmdSplitHandler) Connected(broker proto.MuxBroker, config plugin.Config) {
	log.Infof("plugin connected; config: %v", config)
	t.broker = broker
	t.pconfig = config
}

func (t *cmdSplitHandler) closeHandler() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.h == nil {
		return false
	}

	defer func() {
		t.h = nil
	}()

	if closer, ok := t.h.(io.Closer); ok {
		t.mu.Unlock()
		defer t.mu.Lock()
		err := closer.Close()
		if err != nil {
			log.Errorf("error closing plugin: %v", err)
		}
	}
	return true
}

func (t *cmdSplitHandler) cleanWindow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.win == nil {
		return false
	}

	defer func() {
		t.win = nil
	}()

	t.mu.Unlock()
	defer t.mu.Lock()

	t.closeWindow()

	return true
}

func (t *cmdSplitHandler) closeWindow() {
	err := t.win.Close()
	if err != nil {
		log.Errorf("error closing plugin window: %s", err)
	}
}

func (t *cmdSplitHandler) exitClean() {
	log.Info("received exit signal; cleaning resources...")
	if t.cleanWindow() {
		log.Debug("cleaned window")
	}
	if t.closeHandler() {
		log.Debug("closed handler")
	}
}

func (t *cmdSplitHandler) openSplitWindow() {
	t.mu.Lock()
	win := t.win
	wm := t.wm
	t.mu.Unlock()

	if win != nil {
		log.Debug("received cmd event but win is already open")
		return
	}
	if wm == nil {
		log.Warn("could not handle cmd event: could not resolve wm permission")
		return
	}

	focus, err := wm.Focus()
	if err != nil {
		log.Errorf("failed to get focus: %s", err)
		return
	}

	h, err := t.config.Handler(t.grants, t.broker, focus, t.pconfig)
	if err != nil {
		log.Errorf("error building window handler: %v", err)
		return
	}

	win, err = t.wm.Split(t.config.SplitOrientation, browser.FuncHandler(h, t.exitClean))
	if err != nil {
		log.Errorf("error opening new window: %s", err)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.h = h
	t.win = win
}

func (t *cmdSplitHandler) HandleCommand(cmd editor.Command) (exit bool) {
	if cmd.Name == t.config.Command {
		t.openSplitWindow()
	}
	return false
}

func (t *cmdSplitHandler) PermissionGranted(grants []plugin.Grant) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var err error
	log.Infof("permissions granted: %+v", grants)

	for _, g := range grants {
		switch g.Permission {
		case plugin.PermissionBrowserWindowManager:
			t.wm, err = plugin.WindowManager(g.Token, t.broker)
		case plugin.PermissionEditor:
			t.ed, err = plugin.Editor(g.Token, t.broker)
			if err == nil && t.config.Command != "" {
				err = t.ed.SubscribeCommand(t.config.Command, t)
			}
		}
		if err != nil {
			log.Errorf("PermissionGranted: %+v: %s", g.Permission, err)
		}
	}

	t.grants = grants
}

func (t *cmdSplitHandler) PermissionDenied(perms []plugin.Permission) {
	log.Warnf("permission denied: %v", perms)
}

func (t *cmdSplitHandler) Shutdown(reason string) error {
	log.Warningf("plugin being shutdown: %s", reason)
	t.closeHandler()
	t.cleanWindow()
	return nil
}

func (t *cmdSplitHandler) Health() error {
	return nil
}

// CommandSplitHandlerConfig provides the configuration required to
// use CommandSplitHandler plugin.Grantee helper. See CommandSplitHandler
// for more details.
type CommandSplitHandlerConfig struct {
	// Command that triggers Split
	Command string

	// SplitOrientatio of the new split window.
	SplitOrientation browser.Orientation

	// Handler is the constructor used to install a handler
	// on the split window. The focus argument represents
	// the window in focus when cmd event was fired.
	// If returned Handler satisfies io.Closer, then Close will be called
	// when split window is closed.
	Handler func([]plugin.Grant, proto.MuxBroker, browser.Window,
		plugin.Config) (tui.Handler, error)

	// Permissions to be requested for Handler.
	Permissions []plugin.Permission
}

// ServeCommandSplitHandler serves a plugin.Grantee that opens a split window
// with a new handler when cmd event is fired or command called.
// This function never returns.
func ServeCommandSplitHandler(config CommandSplitHandlerConfig) {
	if config.Handler == nil || config.Command == "" {
		panic(fmt.Sprintf("invalid cmd split handler configuration: "+
			"Handler and Command must be set: %#v", config))
	}

	perms := []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionEditor,
	}
	perms = append(perms, config.Permissions...)
	plugin.Serve(&cmdSplitHandler{config: config}, perms...)
}
