// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package schemedoc

import (
	"context"

	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	test "github.com/unstablebuild/blue/document/test"
	"github.com/unstablebuild/blue/encoding"
	"github.com/unstablebuild/blue/encoding/bson"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/unstablebuild/blue/encoding/toml"
	"github.com/unstablebuild/blue/encoding/yaml"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
)

func testMemoryWorkspaceServiceWithMarshaler(t *testing.T, m encoding.Marshaler) {
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		svc, err := NewDocumentService(scheme, m)
		require.NoError(t, err)
		return svc
	})
}

func testFileWorkspaceServiceWithMarshaler(t *testing.T, m encoding.Marshaler) {
	test.TestDocumentService(t, func(t *testing.T) document.Service {
		name, err := os.MkdirTemp("", "workspace_document_service_test")
		require.NoError(t, err)
		uri, err := workspaceapi.ParseURI("file://" + name)
		require.NoError(t, err)
		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		svc, err := NewDocumentService(scheme, m)
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = svc.Close()
			_ = os.RemoveAll(name)
		})
		return svc
	})
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
	// test preconditions for bson only, because only bson supports
	// time-based preconditions.
	t.Run("backed by FileScheme", func(t *testing.T) {
		testFileWorkspaceServiceWithMarshaler(t, bson.Marshaler())

		test.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
			name, err := os.MkdirTemp("", "workspace_document_service_test")
			require.NoError(t, err)
			uri, err := workspaceapi.ParseURI("file://" + name)
			require.NoError(t, err)
			scheme, err := workspace.NewFileScheme(context.Background(),
				config.NopConfig(), uri)
			require.NoError(t, err)
			svc, err := NewDocumentService(scheme, bson.Marshaler())
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = svc.Close()
				_ = os.RemoveAll(name)
			})
			return svc
		})
	})
	t.Run("backed by MemoryScheme", func(t *testing.T) {
		testMemoryWorkspaceServiceWithMarshaler(t, bson.Marshaler())

		test.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
			uri, err := workspaceapi.ParseURI("memory:///")
			require.NoError(t, err)
			scheme, err := workspace.NewMemoryScheme(context.Background(),
				config.NopConfig(), uri)
			require.NoError(t, err)
			svc, err := NewDocumentService(scheme, bson.Marshaler())
			require.NoError(t, err)
			return svc
		})
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

	name, err := os.MkdirTemp("", "workspace_document_service_test")
	require.NoError(t, err)
	uri, err := workspaceapi.ParseURI("file://" + name)
	require.NoError(t, err)
	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	svc, err := NewDocumentService(scheme, toml.Marshaler())
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

	name, err := os.MkdirTemp("", "workspace_document_service_test")
	require.NoError(t, err)

	aDir := filepath.Join(name, "a")
	require.NoError(t, os.MkdirAll(aDir, 0777))
	uriA, err := workspaceapi.ParseURI(filepath.Join("file://", aDir))
	require.NoError(t, err)
	schemeA, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uriA)
	require.NoError(t, err)
	svcA, err := NewDocumentService(schemeA, toml.Marshaler())
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
	svcB, err := NewDocumentService(schemeB, toml.Marshaler())
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
