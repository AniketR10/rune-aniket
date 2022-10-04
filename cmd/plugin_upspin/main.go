package main

import (
	"net/http"
	_ "net/http/pprof"
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
)

var (
	requiredPermissions = []plugin.Permission{
		plugin.PermissionSchemeManager,
	}
)

type upspinGrantee struct {
	mu     sync.Mutex
	broker proto.MuxBroker
}

func (e *upspinGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Infof("plugin connected; config: %#v", pconfig)
	e.broker = broker
}

func (e *upspinGrantee) PermissionGranted(grants []plugin.Grant) {
	log.Infof("permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case plugin.PermissionSchemeManager:
			m, err := plugin.SchemeManager(g.Token, e.broker)
			if err != nil {
				log.Fatalf("PermissionGranted: %+v: %s", g.Permission, err)
			}
			err = m.RegisterScheme(upspinScheme, newScheme)
			if err != nil {
				log.Fatalf("Could not register scheme:  %s", err)
			}
		}
	}

}

func (e *upspinGrantee) PermissionDenied(perms []plugin.Permission) {
	log.Fatalf("Could not start plugin due to missing permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *upspinGrantee) Shutdown(reason string) error {
	log.Debugf("plugin being shutdown: %s", reason)
	return nil
}

func (e *upspinGrantee) Health() error {
	return nil
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:4568", nil))
	}()

	s := upspinGrantee{}
	plugin.Serve(&s, requiredPermissions...)
}
