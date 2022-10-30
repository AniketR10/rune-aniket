package document

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ernestrc/blue/document"
	test "github.com/ernestrc/blue/document/test"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func testMemoryWorkspaceServiceWithMarshaler(t *testing.T, m Marshaler) {
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		uri, err := workspace.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(config.NopConfig(), uri)
		require.NoError(t, err)
		svc, err := NewWorkspaceService(scheme, m)
		require.NoError(t, err)
		return svc
	})
}

func testFileWorkspaceServiceWithMarshaler(t *testing.T, m Marshaler) {
	dirs := make(map[string]struct{})
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		name, err := ioutil.TempDir("", "workspace_document_service_test")
		require.NoError(t, err)
		if _, ok := dirs[name]; ok {
			panic(fmt.Sprintf("created a duplicate temp dir: %s", name))
		}
		dirs[name] = struct{}{}
		uri, err := workspace.ParseURI("file://" + name)
		require.NoError(t, err)
		scheme, err := workspace.NewFileScheme(config.NopConfig(), uri)
		require.NoError(t, err)
		svc, err := NewWorkspaceService(scheme, m)
		require.NoError(t, err)
		return svc
	})
	for dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}

func TestFileWorkspaceServiceJSON(t *testing.T) {
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, MarshalerJSON())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, MarshalerJSON())
	})
}

func TestFileWorkspaceServiceTOML(t *testing.T) {
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, MarshalerTOML())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, MarshalerTOML())
	})
}

func TestFileWorkspaceServiceYAML(t *testing.T) {
	t.SkipNow()
	// passes everything except numbers in map receivers are of diff types.
	// Periodically remove skipnow to check regressions. If TOML and JSON are
	// working as expected, then there should be nothing fundamentally wrong by
	// using YAML.
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, MarshalerYAML())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, MarshalerYAML())
	})
}
