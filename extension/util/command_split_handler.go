package util

import (
	"context"
	"errors"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	textextension "unstable.build/go-tui/api/text/extension"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
)

// CommandSplitHandlerConfig provides the configuration required to
// use CommandSplitHandler extension.Grantee helper. See CommandSplitHandler
// for more details.
type CommandSplitHandlerConfig struct {
	// Command that triggers Split
	Command string

	// SplitOrientatio of the new split window.
	SplitOrientation browserapi.Orientation

	// Handler is the constructor used to install a handler
	// on the split window. The focus argument represents
	// the window in focus when cmd event was fired.
	// If returned Handler satisfies io.Closer, then Close will be called
	// when split window is closed.
	Handler func(textapi.Command, []extension.Grant, proto.MuxBroker, browserapi.Window,
		config.Config) (browserapi.Handler, error)

	// Permissions to be requested for Handler.
	Permissions []extension.Permission
}

// NewCommandSplitHandler returns a extension.Grantee that opens a split window
// with a new handler when cmd event is fired or command called.
// This function never returns.
func NewCommandSplitHandler(config CommandSplitHandlerConfig) (extension.Grantee, []extension.Permission) {
	if config.Handler == nil || config.Command == "" {
		panic(fmt.Sprintf("invalid cmd split handler configuration: "+
			"Handler and Command must be set: %#v", config))
	}

	perms := []extension.Permission{
		extension.Permission(extension.PermissionBrowserWindowManager),
		extension.Permission(extension.PermissionEditor),
	}
	perms = append(perms, config.Permissions...)
	return &cmdSplitHandler{config: config}, perms
}

type cmdSplitHandler struct {
	mu     sync.Mutex
	config CommandSplitHandlerConfig

	broker  proto.MuxBroker
	wm      browserapi.WindowManager
	ed      textapi.Editor
	pconfig config.Config
	grants  []extension.Grant
	h       browserapi.Handler
	win     browserapi.Window
}

func (t *cmdSplitHandler) Connected(broker proto.MuxBroker, config config.Config) {
	log.Debugf("extension connected; config: %v", config)
	t.broker = broker
	t.pconfig = config
}

func (t *cmdSplitHandler) closeHandler() bool {
	t.mu.Lock()
	h := t.h
	t.h = nil
	t.mu.Unlock()

	if h == nil {
		return false
	}

	err := h.Close()
	if err != nil {
		log.Errorf("command split handler close: %v", err)
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
		log.Errorf("closing extension window: %s", err)
	}
}

func (t *cmdSplitHandler) exitClean() error {
	log.Info("received exit signal; cleaning resources...")
	if t.cleanWindow() {
		log.Debug("cleaned window")
	}
	if t.closeHandler() {
		log.Debug("closed handler")
	}
	return nil
}

func (t *cmdSplitHandler) openSplitWindow(cmd textapi.Command) error {
	focusWin := cmd.Window
	t.mu.Lock()
	win := t.win
	wm := t.wm
	t.mu.Unlock()

	if win != nil {
		log.Debug("received cmd event but win is already open")
		return nil
	}
	if wm == nil {
		return errors.New("insufficient permissions: WindowManager permission was denied")
	}

	h, err := t.config.Handler(cmd, t.grants, t.broker, focusWin, t.pconfig)
	if err != nil {
		err = fmt.Errorf("config.Handler: %v", err)
		return err
	}

	win, err = t.wm.Split(t.config.SplitOrientation, focusWin, browserapi.FuncHandler(h, t.exitClean))
	if err != nil {
		err = fmt.Errorf("wm.Split: %s", err)
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.h = h
	t.win = win
	return nil
}

func (t *cmdSplitHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (exit bool, err error) {
	if cmd.Name == t.config.Command {
		err = t.openSplitWindow(cmd)
		if err != nil {
			log.Error(err)
		}
		return false, err
	}
	return false, nil
}

func (t *cmdSplitHandler) PermissionGranted(grants []extension.Grant) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var err error
	log.Debugf("permissions granted: %+v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionBrowserWindowManager:
			t.wm, err = browserextension.WindowManager(g, t.broker)
		case extension.PermissionEditor:
			t.ed, err = textextension.Editor(g, t.broker)
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

func (t *cmdSplitHandler) PermissionDenied(perms []extension.Permission) {
	log.Warnf("permission denied: %v", perms)
}

func (t *cmdSplitHandler) Shutdown(reason string) error {
	log.Debugf("extension being shutdown: %s", reason)
	t.closeHandler()
	t.cleanWindow()
	return nil
}

func (t *cmdSplitHandler) Health() error {
	return nil
}
