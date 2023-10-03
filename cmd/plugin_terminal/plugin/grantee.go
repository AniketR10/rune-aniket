package plugin

import (
	"context"
	"errors"
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	textplugin "unstable.build/go-tui/api/text/plugin"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

const (
	cmdSplitWindowTerminal = "splitWindowTerminal"
	cmdTerminalTab         = "newTerminal"
)

var (
	requiredPermissions = []plugin.Permission{
		plugin.Permission(plugin.PermissionBrowserWindowManager),
		plugin.Permission(plugin.PermissionEditor),
		plugin.Permission(plugin.PermissionFileSystem),
		plugin.Permission(plugin.PermissionTerminal),
		plugin.Permission(plugin.PermissionBrowserEventPublisher),
		plugin.Permission(plugin.PermissionBrowserNotifications),
	}
	commands = []string{
		cmdSplitWindowTerminal,
		cmdTerminalTab,
	}

	defaultSelectionAttr = term.Attributes{Fg: term.AttrReverse, Bg: term.AttrReverse}
)

// Grantee returns this plugin's Grantee and the permissoins required to run it.
func Grantee() (plugin.Grantee, []plugin.Permission) {
	// log unhandled signals for debugging
	ch := make(chan os.Signal, 1)
	quitch := make(chan struct{})
	signal.Notify(ch)
	go func() {
		for {
			select {
			case sig := <-ch:
				switch sig {
				case syscall.SIGTERM, syscall.SIGKILL:
					log.Info("Received kill signal: exiting")
					os.Exit(1)
				case syscall.SIGWINCH:
					/* received when window changed in size, don't need to do anything */
				case syscall.SIGURG:
					/* received when socket urgent data is ready to be read */
				default:
					log.Debugf("Received unhandled signal: %#v", sig)
				}
			case <-quitch:
				signal.Reset()
				return
			}
		}
	}()

	s := &emulatorGrantee{ch: quitch}
	return s, requiredPermissions
}

type emulatorGrantee struct {
	ch     chan struct{}
	mu     sync.Mutex
	broker proto.MuxBroker

	fs  workspaceapi.FileSystem
	tty workspaceapi.Terminal
	wm  browserapi.WindowManager
	p   browserapi.EventPublisher
	ed  textapi.Editor
	m   browserapi.Notifications

	defAttr       term.Attributes
	selectionAttr term.Attributes
	shell         string
	initialCmd    string
}

func (e *emulatorGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Debugf("plugin connected; config: %#v", pconfig)
	e.broker = broker

	attr, err := config.GetAttributes(pconfig, "attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("coud not read 'attr' property: %v", err)
		return
	}
	e.defAttr = attr

	attr, err = config.GetAttributes(pconfig, "selection_attr")
	if err != nil {
		if err != config.ErrNotFound {
			log.Errorf("coud not read 'selection_attr' property: %v", err)
			return
		}
		attr = defaultSelectionAttr
	}
	e.selectionAttr = attr

	shell, err := pconfig.GetString("shell")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("coud not read 'shell' property: %v", err)
		return
	}
	e.shell = shell

	initialCmd, err := pconfig.GetString("cmd")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("coud not read 'initalCmd' property: %v", err)
		return
	}
	e.initialCmd = initialCmd
}

func (e *emulatorGrantee) PermissionGranted(grants []plugin.Grant) {
	log.Debugf("permissions granted: %v", grants)

	var err error
	for _, g := range grants {
		switch g.Permission {
		case plugin.Permission(plugin.PermissionBrowserEventPublisher):
			e.p, err = browserplugin.EventPublisher(g, e.broker)
		case plugin.Permission(plugin.PermissionBrowserWindowManager):
			e.wm, err = browserplugin.WindowManager(g, e.broker)
		case plugin.Permission(plugin.PermissionTerminal):
			e.tty, err = workspaceplugin.Terminal(g, e.broker)
		case plugin.Permission(plugin.PermissionFileSystem):
			e.fs, err = workspaceplugin.FileSystem(g, e.broker)
		case plugin.Permission(plugin.PermissionBrowserNotifications):
			e.m, err = browserplugin.Notifications(g, e.broker)
		case plugin.Permission(plugin.PermissionEditor):
			e.ed, err = textplugin.Editor(g, e.broker)
			if err == nil {
				for _, cmd := range commands {
					subsErr := e.ed.SubscribeCommand(cmd, e)
					if subsErr != nil {
						err = multierr.Append(err, subsErr)
					}
				}
			}
		}
		if err != nil {
			log.Errorf("PermissionGranted: %+v: %s", g.Permission, err)
		}
	}
}

func (e *emulatorGrantee) PermissionDenied(perms []plugin.Permission) {
	log.Warningf("missing critical permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *emulatorGrantee) Shutdown(reason string) error {
	log.Debugf("plugin being shutdown: %s", reason)
	close(e.ch)
	return nil
}

func (e *emulatorGrantee) Health() error {
	return nil
}

func (e *emulatorGrantee) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (bool, error) {
	exit, err := e.handleCommand(ctx, cmd)
	if err != nil {
		log.Error(err)
	}
	return exit, err
}

func (e *emulatorGrantee) handleCommand(
	ctx context.Context, cmd textapi.Command,
) (bool, error) {
	if e.wm == nil || e.fs == nil || e.tty == nil || e.p == nil || e.ed == nil {
		return false, errors.New("missing critical permissions")
	}
	switch cmd.Name {
	case cmdSplitWindowTerminal, cmdTerminalTab:
	default:
		panic("unknown command")
	}

	log.Tracef("HandleCommand: creating new emulator handler")
	h, err := newEmulator(e.wm, e.tty, e.fs, e.p, e.m, e.shell,
		e.initialCmd, e.defAttr, e.selectionAttr)
	if err != nil {
		err = fmt.Errorf("newEmulator: %s", err)
		return false, err
	}

	uri, err := h.URI()
	if err != nil {
		_ = h.Close()
		return false, err
	}
	t, err := e.wm.Tab(uri, h.Title(), h)
	if err != nil {
		_ = h.Close()
		return false, fmt.Errorf("wm.Tab: %s", err)
	}

	log.Tracef("HandleCommand: created new emulator handler: %p", h)

	switch cmd.Name {
	case cmdSplitWindowTerminal:
		_, err = e.wm.Split(browserapi.OrientationDefault, cmd.Window, t)
	case cmdTerminalTab:
		err = cmd.Window.SetContent(t)
	}
	if err != nil {
		_ = t.Close()
		_ = h.Close()
		return false, err
	}
	return false, nil
}
