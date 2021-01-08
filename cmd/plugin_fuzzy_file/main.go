package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

type fuzzyFinderPlugin struct {
	broker proto.MuxBroker
	wm     browser.WindowManager
	f      browser.ResourceOpener
	m      browser.Messenger
	s      browser.EventSubscriber
	p      browser.EventPublisher

	h      *fuzzyFinderHandler
	config plugin.Config
	win    browser.Window
	cwd    string
}

func (t *fuzzyFinderPlugin) OnConnected(broker proto.MuxBroker, config plugin.Config) {
	log.Infof("plugin connected; config: %#v", config)
	t.broker = broker
	t.config = config
}

func (t *fuzzyFinderPlugin) closeHandler() bool {
	if t.h == nil {
		return false
	}
	err := t.h.Close()
	if err != nil {
		log.Errorf("error closing fuzzy handler: %v", err)
	}
	t.h = nil
	return true
}

func (t *fuzzyFinderPlugin) cleanWindow() bool {
	if t.win == nil {
		return false
	}
	t.closeWindow()
	t.win = nil
	return true
}

func (t *fuzzyFinderPlugin) closeWindow() {
	err := t.win.Close()
	if err != nil {
		log.Errorf("error closing plugin window: %s", err)
	}
}

func (t *fuzzyFinderPlugin) exitClean() {
	log.Info("received exit signal; cleaning resources...")
	if t.cleanWindow() {
		log.Debug("cleaned window")
	}
	if t.closeHandler() {
		log.Debug("closed handler")
	}
}

func (t *fuzzyFinderPlugin) handleInitKey() {
	if t.win != nil {
		log.Debug("received KeyCtrlP event but win is already open")
		return
	}
	focus, err := t.wm.Focus()
	if err != nil {
		log.Errorf("failed to get focus: %s", err)
		return
	}
	t.h = newFuzzyFinderHandler(t.f, t.p, focus, t.config, t.cwd)
	h := browser.CallbackHandler(t.h, t.exitClean)
	win, err := t.wm.SplitHorizontalBelow(h)
	if err != nil {
		log.Errorf("error opening new window: %s", err)
		return
	}
	t.win = win
	log.Debugf("received KeyCtrlP event and created a win: %#v", t.win)
}

func (t *fuzzyFinderPlugin) Handle(ev term.Event) (exit bool) {
	if ev.Type != term.EventKey {
		return
	}

	if t.wm == nil {
		log.Debug("received event but plugin is not ready yet")
		return
	}

	switch ev.Key {
	case term.KeyCtrlP:
		t.handleInitKey()
	}
	return
}

func (t *fuzzyFinderPlugin) subscribeToEvents() error {
	evs := []term.Event{
		term.Event{Type: term.EventKey, Key: term.KeyCtrlP},
	}
	for _, ev := range evs {
		err := t.s.Subscribe(ev, t)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *fuzzyFinderPlugin) OnPermissionGranted(
	token uint32, perm plugin.Permission,
) {
	var err error
	log.Infof("plugin permission granted: %+v", perm)

	switch perm {
	case plugin.PermissionBrowserWindowManager:
		t.wm, err = plugin.WindowManager(token, t.broker)
	case plugin.PermissionBrowserResourceOpener:
		t.f, err = plugin.ResourceOpener(token, t.broker)
	case plugin.PermissionBrowserMessenger:
		t.m, err = plugin.Messenger(token, t.broker)
	case plugin.PermissionBrowserEventPublisher:
		t.p, err = plugin.EventPublisher(token, t.broker)
	case plugin.PermissionBrowserEventSubscriber:
		t.s, err = plugin.EventSubscriber(token, t.broker)
		if err == nil {
			err = t.subscribeToEvents()
		}
	}
	if err != nil {
		log.Errorf("OnPermissionGranted: %+v: %s", perm, err)
	}
}

func (t *fuzzyFinderPlugin) OnPermissionDenied(perm plugin.Permission) {
	log.Fatalf("plugin permission denied: %+v", perm)
}

func (t *fuzzyFinderPlugin) OnShutdown(reason string) error {
	log.Warningf("plugin being shutdown: %s", reason)
	t.closeHandler()
	return nil
}

func (t *fuzzyFinderPlugin) Health() error {
	log.Debug("health check OK")
	return nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)
	plugin.SetLoggingLevel(log.DebugLevel)
	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	dir = fmt.Sprintf("%s/", filepath.Base(dir))

	fuzzyFinder := &fuzzyFinderPlugin{cwd: dir}

	plugin.Serve(fuzzyFinder,
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionBrowserEventSubscriber,
		plugin.PermissionBrowserEventPublisher,
	)
}
