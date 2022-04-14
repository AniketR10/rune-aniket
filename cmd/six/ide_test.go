package main

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/ernestrc/go-tui/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestFiles(t *testing.T) (*os.File, *os.File) {
	configFile, err := ioutil.TempFile("", "six_ide_test")
	require.NoError(t, err)
	require.NoError(t, configFile.Close())

	file, err := ioutil.TempFile("", "six_ide_test")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	return configFile, file
}

func TestIDEInitializationIntegration(t *testing.T) {
	t.Run("does not panic with sample config", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)
		file2, err := ioutil.TempFile("", "six_ide_test")
		require.NoError(t, err)
		require.NoError(t, file2.Close())

		err = ioutil.WriteFile(configFile.Name(), []byte(sampleConfig), 0666)
		require.NoError(t, err)

		cwdURI, err := workspace.CurrentUserHostURI(".")
		require.NoError(t, err)

		i := new(IDE)
		err = i.init(false, cwdURI.String(), configFile.Name(), "", file1.Name(), file2.Name())
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("does not panic with empty config", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)

		err := ioutil.WriteFile(configFile.Name(), []byte("{}"), 0666)
		require.NoError(t, err)

		cwdURI, err := workspace.CurrentUserHostURI(".")
		require.NoError(t, err)

		i := new(IDE)
		err = i.init(false, cwdURI.String(), configFile.Name(), "", file1.Name())
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("overrides config with local wd config", func(t *testing.T) {
		configFile, file1 := makeTestFiles(t)

		config := `
vi:
    search_attr:
        bg: red
        fg: 219
    debug: true
    wrap: true
    smtg_else:
        hello: world
`
		localConfig := `
vi:
    search_attr:
        bg: yellow
    wrap:
      - yes
    smtg_else: true
    extra_key: extra_value
`
		cwd, err := ioutil.TempDir("", "six_ide_local_config_test")
		require.NoError(t, err)

		err = ioutil.WriteFile(configFile.Name(), []byte(config), 0666)
		require.NoError(t, err)

		localConfigFile := path.Join(cwd, ".sixrc")
		err = ioutil.WriteFile(localConfigFile, []byte(localConfig), 0666)
		require.NoError(t, err)

		cwdURI, err := workspace.CurrentUserHostURI(cwd)
		require.NoError(t, err)

		i := new(IDE)

		// sut
		err = i.init(false, cwdURI.String(), configFile.Name(), "", file1.Name())
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"vi": map[string]interface{}{
				"search_attr": map[string]interface{}{
					"bg": "yellow",
					"fg": 219,
				},
				"debug": true,
				"wrap": []interface{}{
					"yes",
				},
				"extra_key": "extra_value",
				"smtg_else": true,
			},
		}, i.cfg)
		// end of sut

		assert.NoError(t, i.closeResources())
	})
}
