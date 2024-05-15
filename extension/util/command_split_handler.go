package util

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	textextension "unstable.build/go-tui/api/text/extension"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
)

// CommandSplitHandlerConfig provides the configuration required to
// use CommandSplitHandler extension.Grantee helper. See CommandSplitHandler
// for more details.
type CommandSplitHandlerConfig struct {
	// Command that triggers Split
	Command textapi.CommandManual

	// SplitOrientatio of the new split window.
	SplitOrientation browserapi.Orientation

	// Handler is the constructor used to install a handler
	// on the split window. The focus argument represents
	// the window in focus when cmd event was fired.
	// If returned Handler satisfies io.Closer, then Close will be called
	// when split window is closed.
	Handler func(context.Context, textapi.Command, []extension.Grant,
		rpc.MuxBroker, browserapi.Window, config.Config) (browserapi.Handler, error)

	// Permissions to be requested for Handler.
	Permissions []extension.Permission
}

// NewCommandSplitHandler returns a extension.Grantee that opens a split window
// with a new handler when cmd event is fired or command called.
// This function never returns.
func NewCommandSplitHandler(config CommandSplitHandlerConfig) (extension.Grantee, []extension.Permission) {
	if config.Handler == nil || config.Command.Name == "" {
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

	broker  rpc.MuxBroker
	wm      browserapi.WindowManager
	ed      textapi.Editor
	pconfig config.Config
	grants  []extension.Grant
	h       browserapi.Handler
	win     browserapi.Window
}

func (t *cmdSplitHandler) Connected(
	ctx context.Context, broker rpc.MuxBroker, config config.Config,
) error {
	t.log(log.DebugLevel, "extension connected")
	t.broker = broker
	t.pconfig = config
	return nil
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
		t.log(log.ErrorLevel, "command split handler close: %v", err)
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
		t.log(log.ErrorLevel, "closing extension window: %v", err)
	}
}

func (t *cmdSplitHandler) exitClean() error {
	t.log(log.DebugLevel, "received exit signal; cleaning resources...")

	if t.cleanWindow() {
		t.log(log.DebugLevel, "cleaned window")
	}
	if t.closeHandler() {
		t.log(log.DebugLevel, "closed handler")
	}
	return nil
}

func (t *cmdSplitHandler) openSplitWindow(ctx context.Context, cmd textapi.Command) error {
	focusWin := cmd.Window
	t.mu.Lock()
	win := t.win
	wm := t.wm
	t.mu.Unlock()

	if win != nil {
		t.log(log.DebugLevel, "received cmd event but win is already open")
		return nil
	}

	h, err := t.config.Handler(ctx, cmd, t.grants, t.broker, focusWin, t.pconfig)
	if err != nil {
		err = fmt.Errorf("config.Handler: %w", err)
		return err
	}

	win, err = wm.Split(t.config.SplitOrientation, focusWin, browserapi.FuncHandler(h, t.exitClean))
	if err != nil {
		err = fmt.Errorf("wm.Split: %w", err)
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.h = h
	t.win = win
	return nil
}

func (t *cmdSplitHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	return iterator.FromSlice[string](nil), nil
}

func (t *cmdSplitHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (exit bool, err error) {
	if cmd.Name == t.config.Command.Name {
		return false, t.openSplitWindow(ctx, cmd)
	}
	return false, nil
}

func (t *cmdSplitHandler) PermissionGranted(ctx context.Context, grants []extension.Grant) (ret error) {
	t.log(log.DebugLevel, "permissions granted: %+v", grants)

	t.mu.Lock()
	defer t.mu.Unlock()

	var err error
	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionBrowserWindowManager:
			t.wm, err = browserextension.WindowManager(ctx, g, t.broker)
		case extension.PermissionEditor:
			t.ed, err = textextension.Editor(ctx, g, t.broker)
			if err == nil {
				err = t.ed.SubscribeCommand(t.config.Command, t)
			}
		}
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}

	t.grants = grants
	return ret
}

func (t *cmdSplitHandler) PermissionDenied(ctx context.Context, perms []extension.Permission) error {
	t.log(log.DebugLevel, "permission denied: %v", perms)

	for _, perm := range perms {
		switch perm {
		case extension.PermissionBrowserWindowManager, extension.PermissionEditor:
			return errors.New("permission window manager and permission " +
				"editor must be granted for this extension to work")
		}
	}
	return nil
}

func (t *cmdSplitHandler) Shutdown(ctx context.Context, reason string) error {
	t.log(log.DebugLevel, "extension being shutdown: %s", reason)
	t.closeHandler()
	t.cleanWindow()
	return nil
}

func (t *cmdSplitHandler) Health(ctx context.Context) error {
	return nil
}

func (t *cmdSplitHandler) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "extutil.cmdSplitHandler",
	}).Logf(level, msg, args...)
}
