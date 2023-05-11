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

// Plugin represents a built-in plugin executable.
type Plugin struct {
	// ID should be a unique representation of the logical
	// plugin.
	ID string

	// Path is the path to the executable.
	Path string

	// Config is the configuration for the plugin.
	Config config.Config
}

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

// WithPluginsRunner sets the Plugins facility of this IDE.
// The default is no plugin runner.
func WithPluginsRunner(p PluginsRunner) Option {
	return func(opts *options) {
		opts.pluginRunner = p
	}
}

// WithPlugin adds Plugin to the IDE's built-in plugins.
func WithPlugin(p Plugin) Option {
	return func(opts *options) {
		if _, ok := opts.plugins[p.ID]; ok {
			panic("built-in plugin with same ID already registered")
		}
		opts.plugins[p.ID] = p
	}
}

// WithConfigFilename defines the base filename of the IDE configuration.
func WithConfigFilename(filename string) Option {
	return func(opts *options) {
		opts.configFilename = filename
	}
}

type options struct {
	publishEvent   EventPublisher
	pluginRunner   PluginsRunner
	locker         sync.Locker
	plugins        map[string]Plugin
	configFilename string
}

func defaultOptions() options {
	return options{
		publishEvent:   tui.PublishEvent,
		pluginRunner:   nopPlugins{},
		locker:         nopLocker{},
		plugins:        make(map[string]Plugin),
		configFilename: ".iderc",
	}
}

type nopPlugins struct {
}

func (n nopPlugins) WorkspacePluginsRunner(locker sync.Locker,
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
