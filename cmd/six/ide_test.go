package main

import (
	"io/ioutil"
	"testing"

	"github.com/ernestrc/go-tui/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDEInitialization(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		configFile, err := ioutil.TempFile("", "six_ide_test")
		require.NoError(t, err)
		require.NoError(t, configFile.Close())

		file1, err := ioutil.TempFile("", "six_ide_test")
		require.NoError(t, err)
		require.NoError(t, file1.Close())

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
}
