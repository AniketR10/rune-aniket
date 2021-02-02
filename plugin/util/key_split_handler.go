package util

import (
	"io"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

type keySplitHandler struct {
	config KeySplitHandlerConfig

	broker  proto.MuxBroker
	wm      browser.WindowManager
	f       browser.ResourceOpener
	s       browser.EventSubscriber
	p       browser.EventPublisher
	pconfig plugin.Config
	h       tui.Handler
	win     browser.Window
}

func (t *keySplitHandler) OnConnected(broker proto.MuxBroker, config plugin.Config) {
	log.Infof("plugin connected; config: %#v", config)
	t.broker = broker
	t.pconfig = config
}

func (t *keySplitHandler) closeHandler() bool {
	if t.h == nil {
		return false
	}
	if closer, ok := t.h.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			log.Errorf("error closing plugin: %v", err)
		}
	}
	t.h = nil
	return true
}

func (t *keySplitHandler) cleanWindow() bool {
	if t.win == nil {
		return false
	}
	t.closeWindow()
	t.win = nil
	return true
}

func (t *keySplitHandler) closeWindow() {
	err := t.win.Close()
	if err != nil {
		log.Errorf("error closing plugin window: %s", err)
	}
}

func (t *keySplitHandler) exitClean() {
	log.Info("received exit signal; cleaning resources...")
	if t.cleanWindow() {
		log.Debug("cleaned window")
	}
	if t.closeHandler() {
		log.Debug("closed handler")
	}
}

func (t *keySplitHandler) handleKeyEvent() {
	if t.win != nil {
		log.Debug("received key event but win is already open")
		return
	}
	focus, err := t.wm.Focus()
	if err != nil {
		log.Errorf("failed to get focus: %s", err)
		return
	}

	t.h = t.config.Handler(t.f, t.p, focus, t.pconfig)
	h := browser.CallbackHandler(t.h, t.exitClean)
	win, err := t.config.Split(t.wm, h)
	if err != nil {
		log.Errorf("error opening new window: %s", err)
		return
	}
	t.win = win
}

func (t *keySplitHandler) Handle(ev term.Event) (exit bool) {
	if ev == t.config.Key {
		t.handleKeyEvent()
	}
	return
}

func (t *keySplitHandler) subscribeToEvents() error {
	err := t.s.Subscribe(t.config.Key, t)
	if err != nil {
		return err
	}
	return nil
}

func (t *keySplitHandler) OnPermissionGranted(
	token uint32, perm plugin.Permission,
) {
	var err error
	log.Infof("plugin permission granted: %+v", perm)

	switch perm {
	case plugin.PermissionBrowserWindowManager:
		t.wm, err = plugin.WindowManager(token, t.broker)
	case plugin.PermissionBrowserResourceOpener:
		t.f, err = plugin.ResourceOpener(token, t.broker)
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

func (t *keySplitHandler) OnPermissionDenied(perm plugin.Permission) {
	log.Fatalf("plugin permission denied: %+v", perm)
}

func (t *keySplitHandler) OnShutdown(reason string) error {
	log.Warningf("plugin being shutdown: %s", reason)
	t.closeHandler()
	t.cleanWindow()
	return nil
}

func (t *keySplitHandler) Health() error {
	return nil
}

// KeySplitHandlerConfig provides the configuration required to
// use KeySplitHandler plugin.Grantee helper. See KeySplitHandler
// for more details.
type KeySplitHandlerConfig struct {
	Key term.Event

	// Split is one of the following WindowManager split methods:
	//   - SplitVerticalRight(Handler) (Window, error)
	//   - SplitVerticalLeft(Handler) (Window, error)
	//   - SplitHorizontalAbove(Handler) (Window, error)
	//   - SplitHorizontalBelow(Handler) (Window, error)
	Split func(browser.WindowManager, browser.Handler) (browser.Window, error)

	// Handler is the constructor used to install a handler
	// on the split window. The focus argument represents
	// the window in focus when key event was fired.
	// If returned Handler satisfies io.Closer, then Close will be called
	// when split window is closed.
	Handler func(r browser.ResourceOpener, e browser.EventPublisher,
		focus browser.Window, config plugin.Config) tui.Handler
}

// ServeKeySplitHandler serves a plugin.Grantee that opens a split window
// with a new handler when key event is fired. The key subscribed to
// the type of window split and the handler used is configured with config.
// If either is not set, this function panics.
// Note that this function never returns.
func ServeKeySplitHandler(config KeySplitHandlerConfig) {
	if config.Handler == nil || (config.Key == term.Event{}) || config.Split == nil {
		panic("invalid key split handler configuration")
	}
	plugin.Serve(&keySplitHandler{config: config},
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionBrowserEventSubscriber,
		plugin.PermissionBrowserEventPublisher,
	)
}
