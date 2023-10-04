package ide

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/extension"
)

// Option is a configuration option for an IDE.
type Option func(*options)

// Extension represents a built-in extension executable.
type Extension struct {
	// ID should be a unique representation of the logical
	// extension.
	ID string

	// Path is the path to the executable.
	Path string

	// Config is the configuration for the extension.
	Config config.Config
}

// WithLocker returns an option that sets locker
// as the event loop locker to synchronize access
// to resources against extension goroutines.
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

// WithExtensionsRunner sets the Extensions facility of this IDE.
// The default is no extension runner.
func WithExtensionsRunner(p ExtensionsRunner) Option {
	return func(opts *options) {
		opts.extensionRunner = p
	}
}

// WithExtension adds Extension to the IDE's built-in extensions.
func WithExtension(p Extension) Option {
	return func(opts *options) {
		if _, ok := opts.extensions[p.ID]; ok {
			panic("built-in extension with same ID already registered")
		}
		opts.extensions[p.ID] = p
	}
}

// WithConfigFilename defines the base filename of the IDE configuration.
func WithConfigFilename(filename string) Option {
	return func(opts *options) {
		opts.configFilename = filename
	}
}

// WithDefaultWallpaper sets the default wallpaper if the
// user doesn't provide one via rc configuration.
func WithDefaultWallpaper(wallpaper string) Option {
	return func(opts *options) {
		opts.defaultWallpaper = wallpaper
	}
}

type options struct {
	publishEvent     EventPublisher
	extensionRunner  ExtensionsRunner
	locker           sync.Locker
	extensions       map[string]Extension
	configFilename   string
	defaultWallpaper string
}

func defaultOptions() options {
	return options{
		publishEvent:     tui.PublishEvent,
		extensionRunner:  nopExtensions{},
		locker:           nopLocker{},
		extensions:       make(map[string]Extension),
		configFilename:   ".iderc",
		defaultWallpaper: legacyDefaultWallpaper,
	}
}

type nopExtensions struct {
}

func (n nopExtensions) WorkspaceExtensionsRunner(locker sync.Locker,
	uri workspaceapi.URI,
	res map[extension.Permission]extension.ResourceRegistrar,
	dataDir string, notifications browser.Notifications) (extension.Runner, error) {
	return nopExtensionsRunner{}, nil
}

type nopExtensionsRunner struct {
}

func (n nopExtensionsRunner) Run(extensionID, path string, config config.Config) error {
	log.Warnf("attempting to run extension %q but no "+
		"extensions facility has been configured", extensionID)
	return nil
}

func (n nopExtensionsRunner) Close() error {
	return nil
}

type nopLocker struct {
}

func (l nopLocker) Lock() {
}

func (l nopLocker) Unlock() {
}
