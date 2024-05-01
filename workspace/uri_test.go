package workspace

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

func TestDefaultSwapDirectory(t *testing.T) {
	tsuite := []struct {
		file    string
		wantDir string
		wantErr bool
	}{
		{"other:///tmp/a.go", "other:///tmp", false},
		{"file:///a.go", "file:///", false},
		{"file:///tmp/a.go", "file:///tmp", false},
		{"file://tmp/a.go", "file://tmp/", false},
		{"ssh://unstable.build/tmp/a.go", "ssh://unstable.build/tmp", false},
	}

	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("DefaultLocalSwapDirectory of %s", tcase.file), func(t *testing.T) {
			uri, err := workspaceapi.ParseURI(tcase.file)
			require.NoError(t, err)
			out, err := DefaultSwapDirectory(uri)
			if tcase.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tcase.wantDir, out.String())
			}
		})
	}
}

func TestDefaultSwapFile(t *testing.T) {
	tsuite := []struct {
		fileIn       string
		swapDirIn    string
		wantSwapFile string
		wantErr      bool
	}{
		{"other:///tmp/a.go", "other:///tmp", "other:///tmp/.a.go.swp", false},
		{"file:///a.go", "file:///tmp", "file:///tmp/.a.go.swp", false},
		{"file:///a.go", "file:///", "file:///.a.go.swp", false},
		{"file:///tmp/a.go", "file:///tmp", "file:///tmp/.a.go.swp", false},
		{"file:///tmp/a.go", "file:///", "file:///.a.go.swp", false},
		{"file://./tmp/a.go", "file://./", "file://./.a.go.swp", false},
		{"file://./a.go", "file://./tmp", "file://./tmp/.a.go.swp", false},
		{"ssh:///a.go", "ssh://my_host/tmp", "", true},
		{"ssh://my_host/a.go", "ssh:///tmp", "", true},
		{"ssh://my_host/a.go", "ssh://creepy_host/tmp", "", true},
		{"ssh://unstablebuild@my_host/a.go", "ssh://jj.furman@my_host/tmp", "", true},
		{"ssh://user@my_host/a.go", "ssh://user@my_host/tmp", "ssh://user@my_host/tmp/.a.go.swp", false},
		{"ssh://my_host/a.go", "ssh://my_host/tmp", "ssh://my_host/tmp/.a.go.swp", false},
		{"ssh://my_host/./a.go", "ssh://my_host/./tmp", "ssh://my_host/tmp/.a.go.swp", false},
	}

	for i, tcase := range tsuite {
		desc := fmt.Sprintf("DefaultLocalSwapFile %d of %s", i, tcase.fileIn)
		t.Run(desc, func(t *testing.T) {
			uri, err := workspaceapi.ParseURI(tcase.fileIn)
			require.NoError(t, err)
			swapUri, err := workspaceapi.ParseURI(tcase.swapDirIn)
			require.NoError(t, err)

			// sut
			out, err := DefaultSwapFile(swapUri, uri)

			if tcase.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tcase.wantSwapFile, out.String())
			}
		})
	}
}

func TestIsWorkspaceURI(t *testing.T) {
	tsuite := []struct {
		workspaceURI string
		uri          string
		expectedOut  bool
	}{
		{"file:///", "file:///tmp", true},
		{"file:///tmp", "file:///tmp", true},
		{"file:///var", "file:///tmp/file", true}, // different folder but workspace can handle it
		{"file:///var", "file:///var/file", true},
		{"file:///var", "file:///var/dir/dir/dir/file", true},
		{"file:///var/", "file:///var/file", true},
		{"file:///", "ssh:///tmp", false},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			inURI, err := workspaceapi.ParseURI(tcase.uri)
			require.NoError(t, err)

			inWorkspaceURI, err := workspaceapi.ParseURI(tcase.workspaceURI)
			require.NoError(t, err)

			fileScheme, err := newTestFileScheme(inWorkspaceURI)
			require.NoError(t, err)
			inWorkspace := NewSchemeWorkspace(inWorkspaceURI, fileScheme)

			// sut
			actualOut, err := IsWorkspaceURI(inWorkspace, inURI)
			require.NoError(t, err)
			assert.Equal(t, tcase.expectedOut, actualOut)
		})
	}
}
