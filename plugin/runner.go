package plugin

import (
	"io"

	"unstable.build/go-tui/api/config"
)

// Runner abstracts the ability to run and stop plugins.
type Runner interface {
	io.Closer
	Run(pluginID, path string, config config.Config) error
}
