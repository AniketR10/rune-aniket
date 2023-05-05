package util

import (
	"io"
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	textplugin "unstable.build/go-tui/api/text/plugin"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
)

// CommandEventHandler combines EventHandler with CommandHandler.
type CommandEventHandler interface {
	textapi.EventHandler
	textapi.CommandHandler
	io.Closer
}

// CommandEventHandlerFacility abstracts the ability to create new CommandEventHandler.
type CommandEventHandlerFacility func(textapi.Editor, []plugin.Grant,
	proto.MuxBroker, config.Config) (CommandEventHandler, error)

// NewEditorEventHandler returns a plugin.Grantee that simply responds to commands.
// It calls fn to build a CommandEventHandler, subsribes it to events
// editor.Event and registers it as the CommandHandler of cmds.
// It also requests extraPerms, in addition to plugin.PermissionEditor.
// All granted permissions are returned in the fn callback. If one of the
// permissions is denied, the plugin will exit with an error.
func NewEditorEventHandler(
	cmds []string,
	fn CommandEventHandlerFacility,
	events []textapi.EventType,
	extraPerms ...plugin.Permission,
) (plugin.Grantee, []plugin.Permission) {
	perms := []plugin.Permission{
		plugin.Permission(plugin.PermissionEditor),
	}
	perms = append(perms, extraPerms...)
	s := &editorGrantee{evs: events, cmds: cmds, newHandler: fn}
	return s, perms
}

type editorGrantee struct {
	mu         sync.Mutex
	broker     proto.MuxBroker
	ed         textapi.Editor
	handler    CommandEventHandler
	newHandler func(textapi.Editor, []plugin.Grant, proto.MuxBroker, config.Config) (CommandEventHandler, error)
	cmds       []string
	pconfig    config.Config
	err        error
	evs        []textapi.EventType
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

	err = t.ed.SubscribeEvents(t.evs, h)
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
			t.ed, err = textplugin.Editor(grant, t.broker)
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
	log.Warningf("missing permissions: permissions denied: %v", perms)
}

func (t *editorGrantee) Shutdown(reason string) error {
	log.Infof("plugin being shutdown: reason: %s", reason)

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
