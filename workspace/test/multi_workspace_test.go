package test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func TestMultiWorkspace(t *testing.T) {
	tsuite := []struct {
		desc               string
		defURI             workspaceapi.URI
		fileURI            workspaceapi.URI
		recover            bool
		wantError          bool
		expectAddWorkspace workspaceapi.URI
	}{
		{"should load files in the default workspace",
			parseURI(t, "memory:///"), parseURI(t, "memory:///file.txt"), false, false, workspaceapi.URI{}},
		{"should recover files in the default workspace",
			parseURI(t, "memory:///"), parseURI(t, "memory:///file.txt"), true, false, workspaceapi.URI{}},
		{"should load files in a registered non-default workspace and should call AddWorkspace with dir URI",
			parseURI(t, "memory:///"), parseURI(t, "test:///file.txt"), false, false, parseURI(t, "test:///")},
		{"should recover files in a registered non-default workspace and should call AddWorkspace with dir URI",
			parseURI(t, "memory:///"), parseURI(t, "test:///file.txt"), true, false, parseURI(t, "test:///")},
		{"should not load files in a non-registered non-default workspace",
			parseURI(t, "memory:///"), parseURI(t, "nagging:///file.txt"), false, true, workspaceapi.URI{}},
		{"should not recover files in a non-registered non-default workspace",
			parseURI(t, "memory:///"), parseURI(t, "nagging:///file.txt"), true, true, workspaceapi.URI{}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			memScheme, err := workspace.NewMemoryScheme(config.NopConfig(), tcase.defURI)
			require.NoError(t, err)
			cwd := workspace.NewSchemeWorkspace(tcase.defURI, memScheme)
			mockManager := &mockManager{}
			cwd = workspace.Multi(mockManager, cwd, tcase.defURI)

			if tcase.recover {
				swapFile := fmt.Sprintf("%s.swp", tcase.fileURI.Path())
				_, werr := memScheme.Open(swapFile, os.O_CREATE, 0)
				require.Nil(t, werr)
				swapFileURI := parseURI(t, fmt.Sprintf("%s.swp", tcase.fileURI.String()))
				_, err = cwd.Recover(tcase.fileURI, swapFileURI, cell.NewBuffer(), false)
			} else {
				_, err = cwd.Load(tcase.fileURI, cell.NewBuffer(), workspaceapi.Dir(tcase.fileURI), false)
			}
			if tcase.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tcase.expectAddWorkspace != (workspaceapi.URI{}) {
				require.Len(t, mockManager.addWorkspace, 1)
				assert.Equal(t, mockManager.addWorkspace[0], tcase.expectAddWorkspace)
			}
		})
	}
}

type mockManager struct {
	addWorkspace []workspaceapi.URI
}

func (m *mockManager) RegisterScheme(string, workspace.SchemeFunc) error {
	panic("should not be called")
}
func (m *mockManager) AddWorkspace(uri workspaceapi.URI) (workspace.Workspace, error) {
	if uri.Scheme() != "test" {
		return nil, errors.New("not registered")
	}
	m.addWorkspace = append(m.addWorkspace, uri)
	scheme, err := NewNopScheme(uri.Scheme())(config.NopConfig(), uri)
	if err != nil {
		return nil, err
	}
	return workspace.NewSchemeWorkspace(uri, scheme), nil
}
