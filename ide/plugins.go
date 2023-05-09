package ide

import (
	"sync"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
)

// PluginsRunner abstracts the ability to construct plugin.Runner.
type PluginsRunner interface {
	WorkspacePluginsRunner(locker sync.Locker,
		uri workspaceapi.URI,
		res map[plugin.Permission]plugin.ResourceRegistrar,
		dataDir string) (plugin.Runner, error)
}

// FuncPluginsRunner wraps fn to satisfy Plugins by invoking in calls to Runner.
func FuncPluginsRunner(
	fn func(sync.Locker, workspaceapi.URI,
		map[plugin.Permission]plugin.ResourceRegistrar, string) (plugin.Runner, error),
) PluginsRunner {
	return fnPlugins{fn: fn}
}

type fnPlugins struct {
	fn func(sync.Locker, workspaceapi.URI,
		map[plugin.Permission]plugin.ResourceRegistrar, string) (plugin.Runner, error)
}

func (f fnPlugins) WorkspacePluginsRunner(locker sync.Locker,
	uri workspaceapi.URI,
	res map[plugin.Permission]plugin.ResourceRegistrar,
	dataDir string) (plugin.Runner, error) {
	return f.fn(locker, uri, res, dataDir)
}
