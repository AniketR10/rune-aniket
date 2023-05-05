package ide

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/term"
)

func TestIDEInitializationIntegration(t *testing.T) {
	t.Run("does not panic with sample config", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)
		file2, err := ioutil.TempFile("", "six_ide_test")
		require.NoError(t, err)
		require.NoError(t, file2.Close())

		err = ioutil.WriteFile(configFile.Name(), []byte(sampleConfig), 0666)
		require.NoError(t, err)

		cwdURI, err := workspaceapi.CurrentUserHostURI(".")
		require.NoError(t, err)

		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		i := new(IDE)
		err = i.init(cwdURI.String(), configFile.Name(), "",
			dir, nopPublishEvent, FuncPlugins(testRunnerFn),
			new(sync.Mutex), file1.Name(), file2.Name())
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("does not panic with empty config", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)

		err := ioutil.WriteFile(configFile.Name(), []byte("{}"), 0666)
		require.NoError(t, err)

		cwdURI, err := workspaceapi.CurrentUserHostURI(".")
		require.NoError(t, err)

		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		i := new(IDE)
		err = i.init(cwdURI.String(), configFile.Name(), "",
			dir, nopPublishEvent, FuncPlugins(testRunnerFn),
			new(sync.Mutex), file1.Name())
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("takes a non-URI as a workspace", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)

		err := ioutil.WriteFile(configFile.Name(), []byte("{}"), 0666)
		require.NoError(t, err)

		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		i := new(IDE)
		err = i.init(".", configFile.Name(), "",
			dir, nopPublishEvent, FuncPlugins(testRunnerFn), new(sync.Mutex), file1.Name())
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("does not publish an event before run is called", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)
		file2, err := ioutil.TempFile("", "six_ide_test")
		require.NoError(t, err)
		require.NoError(t, file2.Close())

		err = ioutil.WriteFile(configFile.Name(), []byte(sampleConfig), 0666)
		require.NoError(t, err)

		cwdURI, err := workspaceapi.CurrentUserHostURI(".")
		require.NoError(t, err)

		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		var published bool
		i := new(IDE)
		err = i.init(cwdURI.String(), configFile.Name(), "",
			dir, func(ev term.Event) bool {
				published = true
				return false
			}, FuncPlugins(testRunnerFn), new(sync.Mutex), file1.Name(), file2.Name())
		require.NoError(t, err)

		assert.False(t, i.publishEvent(term.Event{}))
		assert.False(t, published)

		assert.NoError(t, i.closeResources())
	})
}

func makeTestFiles(t *testing.T) (*os.File, *os.File) {
	configFile, err := ioutil.TempFile("", "six_ide_test")
	require.NoError(t, err)
	require.NoError(t, configFile.Close())

	file, err := ioutil.TempFile("", "six_ide_test")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	return configFile, file
}

func testRunnerFn(
	locker sync.Locker,
	uri workspaceapi.URI,
	res map[plugin.Permission]plugin.ResourceRegistrar,
	dataDir string) (plugin.Runner, error) {
	return testRunner{}, nil
}

type testRunner struct {
}

func (r testRunner) Run(pluginID, path string, config config.Config) error {
	return nil
}

func (r testRunner) Close() error {
	return nil
}
