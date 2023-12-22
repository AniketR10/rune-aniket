package extension

import (
	"context"
	"fmt"
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

func (e *upspinGrantee) Connected(
	ctx context.Context, broker proto.MuxBroker, pconfig config.Config,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Debugf("extension connected; config: %#v", pconfig)
	e.broker = broker

	return nil
}

func (e *upspinGrantee) PermissionGranted(
	ctx context.Context, grants []extension.Grant,
) error {
	log.Debugf("permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionSchemeManager:
			m, err := schemeextension.SchemeManager(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire scheme manager: %w", err)
			}
			err = m.RegisterScheme(upspinScheme, newScheme)
			if err != nil {
				return fmt.Errorf("register scheme: %w", err)
			}
			// store so finalizer doesn't kill the scheme RPC pipeline
			e.m = m
		}
	}
	return nil
}

func (e *upspinGrantee) PermissionDenied(
	ctx context.Context, perms []extension.Permission,
) error {
	return fmt.Errorf("missing critical permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *upspinGrantee) Shutdown(ctx context.Context, reason string) error {
	log.Debugf("extension being shutdown: %s", reason)
	return nil
}

func (e *upspinGrantee) Health(context.Context) error {
	return nil
}
