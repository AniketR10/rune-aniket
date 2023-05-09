package ide

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
)

// Option is a configuration option for an IDE.
type Option func(*options)

// WithLocker returns an option that sets locker
// as the event loop locker to synchronize access
// to resources against plugin goroutines.
//
// The default is nop locker, so no synchronization.
func WithLocker(locker sync.Locker) Option {
	return func(opts *options) {
		opts.locker = locker
	}
}

// WithPublishEvent sets the EventPublisher of the IDE.
// The default is tui.PublishEvent.
func WithPublishEvent(p EventPublisher) Option {
	return func(opts *options) {
		opts.publishEvent = p
	}
}

// WithPlugins sets the Plugins facility of this IDE.
// The default is no plugin runner.
func WithPlugins(p Plugins) Option {
	return func(opts *options) {
		opts.pluginRunner = p
	}
}

type options struct {
	publishEvent EventPublisher
	pluginRunner Plugins
	locker       sync.Locker
}

func defaultOptions() options {
	return options{
		publishEvent: tui.PublishEvent,
		pluginRunner: nopPlugins{},
		locker:       nopLocker{},
	}
}

type nopPlugins struct {
}

func (n nopPlugins) Runner(locker sync.Locker,
	uri workspaceapi.URI,
	res map[plugin.Permission]plugin.ResourceRegistrar,
	dataDir string) (plugin.Runner, error) {
	return nopPluginsRunner{}, nil
}

type nopPluginsRunner struct {
}

func (n nopPluginsRunner) Run(pluginID, path string, config config.Config) error {
	log.Warnf("attempting to run plugin %q but no "+
		"plugins facility has been configured", pluginID)
	return nil
}

func (n nopPluginsRunner) Close() error {
	return nil
}

type nopLocker struct {
}

func (l nopLocker) Lock() {
}

func (l nopLocker) Unlock() {
}
