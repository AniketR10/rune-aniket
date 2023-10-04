package extension

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	schemeextension "unstable.build/go-tui/api/scheme/extension"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
)

var (
	requiredPermissions = []extension.Permission{
		extension.PermissionSchemeManager,
	}
)

// Grantee returns this extension's Grantee and the permission required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return new(upspinGrantee), requiredPermissions
}

type upspinGrantee struct {
	mu     sync.Mutex
	broker proto.MuxBroker
	m      schemeapi.SchemeManager
}

func (e *upspinGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Debugf("extension connected; config: %#v", pconfig)
	e.broker = broker
}

func (e *upspinGrantee) PermissionGranted(grants []extension.Grant) {
	log.Debugf("permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionSchemeManager:
			m, err := schemeextension.SchemeManager(g, e.broker)
			if err != nil {
				log.Errorf("PermissionGranted: %+v: %s", g.Permission, err)
				continue
			}
			err = m.RegisterScheme(upspinScheme, newScheme)
			if err != nil {
				log.Errorf("Could not register scheme:  %s", err)
				continue
			}
			// store so finalizer doesn't kill the scheme RPC pipeline
			e.m = m
		}
	}

}

func (e *upspinGrantee) PermissionDenied(perms []extension.Permission) {
	log.Warningf("missing critical permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *upspinGrantee) Shutdown(reason string) error {
	log.Debugf("extension being shutdown: %s", reason)
	return nil
}

func (e *upspinGrantee) Health() error {
	return nil
}
