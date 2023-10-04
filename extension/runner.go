package extension

import (
	"io"

	"unstable.build/go-tui/api/config"
)

// Runner abstracts the ability to run and stop extensions.
type Runner interface {
	io.Closer
	Run(extensionID, path string, config config.Config) error
}
