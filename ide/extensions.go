package ide

import (
	"sync"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/extension"
)

// ExtensionsRunner abstracts the ability to construct extension.Runner.
type ExtensionsRunner interface {
	WorkspaceExtensionsRunner(locker sync.Locker,
		uri workspaceapi.URI,
		res map[extension.Permission]extension.ResourceRegistrar,
		dataDir string, notifications browser.Notifications) (extension.Runner, error)
}

// FuncExtensionsRunner wraps fn to satisfy Extensions by invoking in calls to Runner.
func FuncExtensionsRunner(
	fn func(sync.Locker, workspaceapi.URI,
		map[extension.Permission]extension.ResourceRegistrar, string,
		browser.Notifications) (extension.Runner, error),
) ExtensionsRunner {
	return fnExtensions{fn: fn}
}

type fnExtensions struct {
	fn func(sync.Locker, workspaceapi.URI,
		map[extension.Permission]extension.ResourceRegistrar, string,
		browser.Notifications) (extension.Runner, error)
}

func (f fnExtensions) WorkspaceExtensionsRunner(
	locker sync.Locker, uri workspaceapi.URI,
	res map[extension.Permission]extension.ResourceRegistrar,
	dataDir string, n browser.Notifications,
) (extension.Runner, error) {
	return f.fn(locker, uri, res, dataDir, n)
}
