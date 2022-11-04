package workspace

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ernestrc/blue/document"
	test "github.com/ernestrc/blue/document/test"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/ernestrc/blue/encoding/toml"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func testMemoryWorkspaceServiceWithMarshaler(t *testing.T, m encoding.Marshaler) {
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

func testFileWorkspaceServiceWithMarshaler(t *testing.T, m encoding.Marshaler) {
	dirs := make(map[string]document.Service)
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		name, err := ioutil.TempDir("", "workspace_document_service_test")
		require.NoError(t, err)
		if _, ok := dirs[name]; ok {
			panic(fmt.Sprintf("created a duplicate temp dir: %s", name))
		}
		uri, err := workspace.ParseURI("file://" + name)
		require.NoError(t, err)
		scheme, err := workspace.NewFileScheme(config.NopConfig(), uri)
		require.NoError(t, err)
		svc, err := NewWorkspaceService(scheme, m)
		require.NoError(t, err)
		dirs[name] = svc
		return svc
	})
	for dir, svc := range dirs {
		svc.Close()
		_ = os.RemoveAll(dir)
	}
}

func TestFileWorkspaceServiceJSON(t *testing.T) {
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, json.Marshaler())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, json.Marshaler())
	})
}

func TestFileWorkspaceServiceBSON(t *testing.T) {
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, bson.Marshaler())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, bson.Marshaler())
	})
}

func TestFileWorkspaceServiceTOML(t *testing.T) {
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, toml.Marshaler())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, json.Marshaler())
	})
}

func TestFileWorkspaceServiceYAML(t *testing.T) {
	t.SkipNow()
	// passes everything except numbers in map receivers are of diff types.
	// Periodically remove skipnow to check regressions. If TOML and JSON are
	// working as expected, then there should be nothing fundamentally wrong by
	// using YAML.
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, yaml.Marshaler())
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, yaml.Marshaler())
	})
}

func TestSetOverrideIssue(t *testing.T) {
	type testStruct struct {
		Content []string
	}

	name, err := ioutil.TempDir("", "workspace_document_service_test")
	require.NoError(t, err)
	uri, err := workspace.ParseURI("file://" + name)
	require.NoError(t, err)
	scheme, err := workspace.NewFileScheme(config.NopConfig(), uri)
	require.NoError(t, err)
	svc, err := NewWorkspaceService(scheme, toml.Marshaler())
	require.NoError(t, err)

	require.NoError(t, svc.Set(context.Background(), "1234", &testStruct{Content: []string{
		"111111111111111111111111111111111111111111111111111111111111\n",
		"111111111111111111111111111111111111111111111111111111111111\n",
		"22221111111111111111111111111111\n",
	}}))
	var temp testStruct
	require.NoError(t, svc.Get(context.Background(), "1234", &temp))
	require.NoError(t, svc.Set(context.Background(), "1234", &testStruct{Content: []string{"a"}}))
	require.NoError(t, svc.Get(context.Background(), "1234", &temp))
}
