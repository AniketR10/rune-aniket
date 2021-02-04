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

var requiredPermissions = []plugin.Permission{
	plugin.PermissionEditor,
}

type lspGrantee struct {
	broker  proto.MuxBroker
	ed      editor.Editor
	s       browser.EventSubscriber
	handler *lspEditorHandler
	pconfig plugin.Config
	err     error
}

func (t *lspGrantee) Connected(broker proto.MuxBroker, config plugin.Config) {
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
	h, err := newLspHandler(t.ed, t.pconfig)
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

func (t *lspGrantee) PermissionGranted(grants []plugin.Grant) {
	log.Infof("permissions granted: %v", grants)

	for _, grant := range grants {
		var err error
		switch grant.Permission {
		case plugin.PermissionEditor:
			t.ed, err = plugin.Editor(grant.Token, t.broker)
			if err == nil {
				err = t.subscribeToEvents()
			}
		}
		if err != nil {
			log.Errorf("PermissionGranted: %+v: %s", grants, err)
		}
	}
}

func (t *lspGrantee) PermissionDenied(perms []plugin.Permission) {
	log.Fatalf("Could not start plugin due to missing permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (t *lspGrantee) Shutdown(reason string) error {
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

	plugin.Serve(&lspGrantee{}, requiredPermissions...)
}
