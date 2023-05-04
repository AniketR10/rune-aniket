package ide

import (
	"sync"

	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
)

// Plugins abstracts the ability to construct plugin.Runner.
type Plugins interface {
	Runner(locker sync.Locker,
		uri workspaceapi.URI,
		res map[plugin.Permission]plugin.ResourceRegistrar,
		dataDir string) (plugin.Runner, error)
}

// FuncPlugins wraps fn to satisfy Plugins by invoking in calls to Runner.
func FuncPlugins(
	fn func(sync.Locker, workspaceapi.URI,
		map[plugin.Permission]plugin.ResourceRegistrar, string) (plugin.Runner, error),
) Plugins {
	return fnPlugins{fn: fn}
}

type fnPlugins struct {
	fn func(sync.Locker, workspaceapi.URI,
		map[plugin.Permission]plugin.ResourceRegistrar, string) (plugin.Runner, error)
}

func (f fnPlugins) Runner(locker sync.Locker,
	uri workspaceapi.URI,
	res map[plugin.Permission]plugin.ResourceRegistrar,
	dataDir string) (plugin.Runner, error) {
	return f.fn(locker, uri, res, dataDir)
}
