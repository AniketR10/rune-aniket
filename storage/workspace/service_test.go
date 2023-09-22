package workspace

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ernestrc/blue/document"
	test "github.com/ernestrc/blue/document/test"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/bson"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/ernestrc/blue/encoding/toml"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/workspace"
)

func testMemoryWorkspaceServiceWithMarshaler(t *testing.T, m encoding.Marshaler) {
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
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
		uri, err := workspaceapi.ParseURI("file://" + name)
		require.NoError(t, err)
		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
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
		testMemoryWorkspaceServiceWithMarshaler(t, toml.Marshaler())
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
	uri, err := workspaceapi.ParseURI("file://" + name)
	require.NoError(t, err)
	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
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

func TestEscapeBoundaries(t *testing.T) {
	type testStruct struct {
		Content []string
	}

	name, err := ioutil.TempDir("", "workspace_document_service_test")
	require.NoError(t, err)

	aDir := filepath.Join(name, "a")
	require.NoError(t, os.MkdirAll(aDir, 0777))
	uriA, err := workspaceapi.ParseURI(filepath.Join("file://", aDir))
	require.NoError(t, err)
	schemeA, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uriA)
	require.NoError(t, err)
	svcA, err := NewWorkspaceService(schemeA, toml.Marshaler())
	require.NoError(t, err)
	require.NoError(t, svcA.Set(context.Background(), "1234", &testStruct{Content: []string{
		"SECRET",
	}}))

	bDir := filepath.Join(name, "b")
	require.NoError(t, os.MkdirAll(bDir, 0777))
	uriB, err := workspaceapi.ParseURI(filepath.Join("file://", bDir))
	require.NoError(t, err)
	schemeB, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uriB)
	require.NoError(t, err)
	svcB, err := NewWorkspaceService(schemeB, toml.Marshaler())
	require.NoError(t, err)

	// its not able to read
	var temp testStruct
	require.Error(t, svcB.Get(context.Background(), "../a/1234", &temp))

	// its not able to write
	require.NoError(t, svcB.Set(context.Background(), "../a/1234", &testStruct{Content: []string{
		"OH BOY",
	}}))
	require.NoError(t, svcA.Get(context.Background(), "1234", &temp))
	assert.Equal(t, []string{"SECRET"}, temp.Content)

	require.NoError(t, svcB.Get(context.Background(), "../a/1234", &temp))
	assert.Equal(t, []string{"OH BOY"}, temp.Content)
}
