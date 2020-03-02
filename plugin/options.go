package plugin

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// WithLogger returns a ManagerOption which configures a manager
// to use logger.
func WithLogger(logger *log.Logger) Option {
	return func(cfg *managerConfig) {
		cfg.logger = logger
	}
}

// WithHandshakeTimeout returns a ManagerOption which
// configures a manager to timeout plugins if handshake is not
// completed within d.
func WithHandshakeTimeout(d time.Duration) Option {
	return func(cfg *managerConfig) {
		cfg.handshakeTimeout = d
	}
}

// WithHealthTimeout returns a ManagerOption which
// configures a manager to timeout if plugins do not respond
// to health checks within d.
func WithHealthTimeout(d time.Duration) Option {
	return func(cfg *managerConfig) {
		cfg.healthCheckTicker = d
	}
}

// WithHealthRetries returns a ManagerOption which
// configures a manager to try to assert a plugin's health
// up to 1 + retries before giving up.
func WithHealthRetries(retries int) Option {
	return func(cfg *managerConfig) {
		cfg.healthRetries = retries
	}
}
