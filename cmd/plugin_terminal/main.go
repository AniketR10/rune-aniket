package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const (
	cmdSplitWindowTerminal = "splitWindowTerminal"
	cmdTerminalTab         = "newTerminal"
)

var (
	requiredPermissions = []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionEditor,
		plugin.PermissionWorkspace,
		plugin.PermissionBrowserEventPublisher,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionClipboard,
	}
	commands = []string{
		cmdSplitWindowTerminal,
		cmdTerminalTab,
	}

	defaultSelectionAttr = term.Attributes{Fg: term.AttrReverse, Bg: term.AttrReverse}
)

type emulatorGrantee struct {
	ch     chan struct{}
	mu     sync.Mutex
	broker proto.MuxBroker

	wp workspace.API
	wm browser.WindowManager
	p  browser.EventPublisher
	ed text.Editor
	m  browser.Messenger
	c  plugin.Clipboard

	defAttr       term.Attributes
	selectionAttr term.Attributes
	shell         string
	initialCmd    string
}

func (e *emulatorGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Infof("plugin connected; config: %#v", pconfig)
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
	log.Infof("permissions granted: %v", grants)

	var err error
	for _, g := range grants {
		switch g.Permission {
		case plugin.PermissionBrowserEventPublisher:
			e.p, err = plugin.EventPublisher(g.Token, e.broker)
		case plugin.PermissionBrowserWindowManager:
			e.wm, err = plugin.WindowManager(g.Token, e.broker)
		case plugin.PermissionWorkspace:
			e.wp, err = plugin.Workspace(g.Token, e.broker)
		case plugin.PermissionBrowserMessenger:
			e.m, err = plugin.Messenger(g.Token, e.broker)
		case plugin.PermissionEditor:
			e.ed, err = plugin.Editor(g.Token, e.broker)
			if err == nil {
				for _, cmd := range commands {
					subsErr := e.ed.SubscribeCommand(cmd, e)
					if subsErr != nil {
						err = multierr.Append(err, subsErr)
					}
				}
			}
		case plugin.PermissionClipboard:
			e.c, err = plugin.GetClipboard(g.Token, e.broker)
		}
		if err != nil {
			log.Errorf("PermissionGranted: %+v: %s", g.Permission, err)
		}
	}
}

func (e *emulatorGrantee) PermissionDenied(perms []plugin.Permission) {
	if len(perms) == 1 && perms[0] == plugin.PermissionEditor {
		// continue without clibboard
		_ = e.m.SetMessage("plugin_terminal: clipboard permission should be granted for an optimal experience")
		return
	}
	log.Fatalf("Could not start plugin due to missing permissions: "+
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
	ctx context.Context, cmd text.Command,
) (bool, error) {
	exit, err := e.handleCommand(ctx, cmd)
	if err != nil {
		log.Error(err)
	}
	return exit, err
}

func (e *emulatorGrantee) handleCommand(
	ctx context.Context, cmd text.Command,
) (bool, error) {
	switch cmd.Name {
	case cmdSplitWindowTerminal, cmdTerminalTab:
	default:
		panic("unknown command")
	}

	log.Tracef("HandleCommand: creating new emulator handler")
	h, err := newEmulator(e.wm, e.wp, e.p, e.m, e.c, e.shell,
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
		_, err = e.wm.Split(browser.OrientationDefault, t)
	case cmdTerminalTab:
		win, err := e.wm.Focus()
		if err == nil {
			err = win.SetContent(t)
		}
	}
	if err != nil {
		_ = t.Close()
		_ = h.Close()
		return false, err
	}
	return false, nil
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:2268", nil))
	}()

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

	s := emulatorGrantee{ch: quitch}
	plugin.Serve(&s, requiredPermissions...)
}
