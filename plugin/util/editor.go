package util

import (
	"io"
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
)

// CommandEventHandler combines EventHandler with CommandHandler.
type CommandEventHandler interface {
	text.EventHandler
	text.CommandHandler
	io.Closer
}

type editorGrantee struct {
	mu         sync.Mutex
	broker     proto.MuxBroker
	ed         text.Editor
	handler    CommandEventHandler
	newHandler func(text.Editor, []plugin.Grant, proto.MuxBroker, config.Config) (CommandEventHandler, error)
	cmds       []string
	pconfig    config.Config
	err        error
	evs        []text.EventType
}

func (t *editorGrantee) Connected(broker proto.MuxBroker, config config.Config) {
	t.mu.Lock()
	defer t.mu.Unlock()

	log.Infof("plugin connected; config: %#v", config)
	t.broker = broker
	t.pconfig = config
}

func (t *editorGrantee) setNewHandler(grants []plugin.Grant) (CommandEventHandler, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, err := t.newHandler(t.ed, grants, t.broker, t.pconfig)
	if err != nil {
		return nil, err
	}
	t.handler = h
	return h, nil
}

func (t *editorGrantee) subscribeToEvents(grants []plugin.Grant) error {
	h, err := t.setNewHandler(grants)
	if err != nil {
		return err
	}

	err = t.ed.SubscribeEditorEvents(t.evs, h)
	if err != nil {
		return err
	}

	for _, cmd := range t.cmds {
		if err = t.ed.SubscribeCommand(cmd, h); err != nil {
			return err
		}
	}
	log.Debugf("subscribed to events %+v", t.evs)
	return nil
}

func (t *editorGrantee) PermissionGranted(grants []plugin.Grant) {
	log.Infof("permissions granted: %v", grants)

	for _, grant := range grants {
		var err error
		switch grant.Permission {
		case plugin.PermissionEditor:
			t.ed, err = plugin.Editor(grant.Token, t.broker)
			if err == nil {
				err = t.subscribeToEvents(grants)
			}
		}
		if err != nil {
			log.Errorf("PermissionGranted: %+v: %s", grants, err)
		}
	}
}

func (t *editorGrantee) PermissionDenied(perms []plugin.Permission) {
	log.Fatalf("Could not start plugin due to missing permissions: "+
		"denied: %v", perms)
}

func (t *editorGrantee) Shutdown(reason string) error {
	log.Warningf("plugin being shutdown: %s", reason)

	t.mu.Lock()
	handler := t.handler
	t.mu.Unlock()

	if handler != nil {
		return handler.Close()
	}
	return nil
}

func (t *editorGrantee) Health() error {
	return nil
}

// ServeEditorEventHandler calls fn to build a CommandEventHandler,
// subsribes it to events editor.Event and registers it as the CommandHandler
// of cmds. It also requests extraPerms, in addition to plugin.PermissionEditor.
// All granted permissions are returned in the fn callback. If one of the
// permissions is denied, the plugin will exit with an error.
func ServeEditorEventHandler(
	cmds []string,
	fn func(text.Editor, []plugin.Grant, proto.MuxBroker, config.Config) (CommandEventHandler, error),
	events []text.EventType,
	extraPerms ...plugin.Permission,
) {
	perms := []plugin.Permission{
		plugin.PermissionEditor,
	}
	perms = append(perms, extraPerms...)
	s := &editorGrantee{evs: events, cmds: cmds, newHandler: fn}
	plugin.Serve(s, perms...)
}
