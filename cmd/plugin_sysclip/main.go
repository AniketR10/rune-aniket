package main

import (
	"sync"
	"time"

	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"

	"github.com/atotto/clipboard"
	log "github.com/sirupsen/logrus"
)

var (
	requiredPermissions = []plugin.Permission{
		plugin.PermissionClipboard,
	}
)

type systemClipboard struct {
	mu          sync.Mutex
	data        string
	lastUpdated time.Time
	broker      proto.MuxBroker
	registerID  string
}

func (c *systemClipboard) Paste() (string, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	text, err := clipboard.ReadAll()
	if err != nil {
		return "", time.Time{}, err
	}
	// if data didn't come from Copy, then assume
	// it's always the most up to date
	if c.data != text {
		return text, time.Now(), nil
	}
	return text, c.lastUpdated, nil
}

func (c *systemClipboard) Copy(data string, ts time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUpdated = ts
	c.data = data
	return clipboard.WriteAll(data)
}

func (c *systemClipboard) Connected(broker proto.MuxBroker, pconfig config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if clipboard.Unsupported {
		log.Fatal("clipboard package does not support this system: terminating plugin")
	}

	log.Debugf("plugin connected; config: %#v", pconfig)
	c.broker = broker
	c.registerID = text.DefaultRegisterID

	registerID, err := pconfig.GetString("register")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warnf("coud not read 'register' property: %v, using default register", err)
		}
		registerID = text.DefaultRegisterID
	}
	c.registerID = registerID
}

func (c *systemClipboard) PermissionGranted(grants []plugin.Grant) {
	log.Infof("permissions granted: %v", grants)

	grant := grants[0]

	switch grant.Permission {
	case plugin.PermissionClipboard:
		clipboard, err := plugin.GetClipboard(grant.Token, c.broker)
		if err == nil {
			err = clipboard.SetRegister(c.registerID, c)
		}
		if err != nil {
			log.Errorf("PermissionGranted: %+v: %s", grants, err)
		}
	}
}

func (c *systemClipboard) PermissionDenied(perms []plugin.Permission) {
	log.Fatalf("Could not start plugin due to missing permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (c *systemClipboard) Shutdown(reason string) error {
	log.Warningf("plugin being shutdown: %s", reason)
	return nil
}

func (c *systemClipboard) Health() error {
	return nil
}

func main() {
	s := &systemClipboard{}
	plugin.Serve(s, requiredPermissions...)
}
