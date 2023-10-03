package process

import (
	"sync"
	"time"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
)

// WithHandshakeTimeout returns an Option which
// configures a manager to timeout plugins if handshake is not
// completed within d.
func WithHandshakeTimeout(d time.Duration) Option {
	return func(cfg *managerConfig) {
		cfg.handshakeTimeout = d
	}
}

// WithHealthTimeout returns an Option which
// configures a manager to timeout if plugins do not respond
// to health checks within d.
func WithHealthTimeout(d time.Duration) Option {
	return func(cfg *managerConfig) {
		cfg.healthCheckTicker = d
	}
}

// WithHealthRetries returns an Option which
// configures a manager to try to assert a plugin's health
// up to 1 + retries before giving up.
func WithHealthRetries(retries int) Option {
	return func(cfg *managerConfig) {
		cfg.healthRetries = retries
	}
}

// WithLocker returns an Option that configures
// the manager resources sync.Locker to be the given
// locker.
func WithLocker(locker sync.Locker) Option {
	return func(cfg *managerConfig) {
		cfg.locker = locker
	}
}

// WithWorkspace returns an option that configures the
// workspace directory.
func WithWorkspace(uri workspaceapi.URI) Option {
	return func(cfg *managerConfig) {
		cfg.workspace = uri
	}
}

// WithDataDir returns an option that configures the
// storage directory.
func WithDataDir(dataDir string) Option {
	return func(cfg *managerConfig) {
		cfg.dataDir = dataDir
	}
}

// WithNotifications returns an option that configures
// a plugin.Manager's notifications.
func WithNotifications(n browser.Notifications) Option {
	return func(cfg *managerConfig) {
		cfg.notifications = n
	}
}
