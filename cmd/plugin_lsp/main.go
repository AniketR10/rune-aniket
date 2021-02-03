package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
)

type lspGrantee struct {
	broker  proto.MuxBroker
	ed      editor.Editor
	m       browser.Messenger
	p       browser.EventPublisher
	s       browser.EventSubscriber
	handler *lspEditorHandler
	pconfig plugin.Config
	err     error
}

func (t *lspGrantee) OnConnected(broker proto.MuxBroker, config plugin.Config) {
	log.Infof("plugin connected; config: %#v", config)
	t.broker = broker
	t.pconfig = config
}

func (t *lspGrantee) subscribeToEvents() error {
	evs := []editor.EventType{
		editor.EventTypeClose,
		editor.EventTypeFlush,
		editor.EventTypeOpen,
		editor.EventTypeInsert,
		editor.EventTypeDelete,
	}
	h, err := newLspHandler(t.ed, t.p, t.pconfig)
	if err != nil {
		return err
	}
	t.handler = h
	for _, ev := range evs {
		err := t.ed.SubscribeEditor(ev, h)
		if err != nil {
			return err
		}
	}

	cmds := []string{commandNextDiagnostic, commandPrevDiagnostic}
	for _, cmd := range cmds {
		if err = t.ed.Register(cmd, h); err != nil {
			return err
		}
	}
	log.Debugf("subscribed to events %+v", evs)
	return nil
}

func (t *lspGrantee) OnPermissionGranted(
	token uint32, perm plugin.Permission,
) {
	log.Infof("plugin permission granted: %+v", perm)

	var err error
	switch perm {
	case plugin.PermissionBrowserMessenger:
		t.m, err = plugin.Messenger(token, t.broker)
		if err == nil && t.err != nil {
			err = t.m.SetMessage("Error: %v", t.err)
		}
	case plugin.PermissionEditor:
		t.ed, err = plugin.Editor(token, t.broker)
		if err == nil {
			err = t.subscribeToEvents()
		}
	case plugin.PermissionBrowserEventPublisher:
		t.p, err = plugin.EventPublisher(token, t.broker)
		if err == nil && t.err != nil {
			err = t.m.SetMessage("Error: %v", t.err)
		}
	}
	if err != nil {
		log.Errorf("OnPermissionGranted: %+v: %s", perm, err)
		if t.m == nil {
			t.err = err
			return
		}

		err = t.m.SetMessage("Error: %v", t.err)
		if err != nil {
			log.Errorf("SetMessage: failed to set error %v: %v", t.err, err)
		}
	}
}

func (t *lspGrantee) OnPermissionDenied(perm plugin.Permission) {
	log.Fatalf("plugin permission denied: %+v", perm)
}

func (t *lspGrantee) OnShutdown(reason string) error {
	log.Warningf("plugin being shutdown: %s", reason)
	if t.handler != nil {
		return t.handler.Close()
	}
	return nil
}

func (t *lspGrantee) Health() error {
	return nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.TraceLevel)
	plugin.SetLoggingLevel(log.TraceLevel)

	go func() {
		log.Println(http.ListenAndServe("localhost:6063", nil))
	}()

	plugin.Serve(&lspGrantee{},
		plugin.PermissionBrowserEventPublisher,
		plugin.PermissionEditor,
		plugin.PermissionBrowserMessenger,
	)
}
