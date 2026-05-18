// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.

package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestDocsSchemePrefill(t *testing.T) {
	uri, err := workspaceapi.ParseURI("docs:///")
	require.NoError(t, err)

	s, err := newDocsSchemeFunc()(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	// intro.md is one of the embedded files; opening it should yield non-empty content.
	f, err := s.Open("/intro.md")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	data, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.NotEmpty(t, data, "embedded intro.md should not be empty")
}

// TestDocsSchemeAllFilesNonEmpty exercises every embedded markdown file via
// the docs scheme and asserts that the in-memory copy carries the same bytes
// as the source embed at the same relative path. The scheme preserves the
// embedded directory layout (e.g. /develop/sdk.md), so opening by relative
// path must round-trip the embed contents.
func TestDocsSchemeAllFilesNonEmpty(t *testing.T) {
	uri, err := workspaceapi.ParseURI("docs:///")
	require.NoError(t, err)

	s, err := newDocsSchemeFunc()(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	expected := make(map[string][]byte)
	require.NoError(t, fsWalkMD(t, &expected))
	require.NotEmpty(t, expected)

	for rel, want := range expected {
		p := "/" + rel
		f, err := s.Open(p)
		require.NoError(t, err, "open %s", p)

		got, err := io.ReadAll(f)
		_ = f.Close()
		require.NoError(t, err, "read %s", p)

		assert.Equal(t, string(want), string(got), "%s contents mismatch", p)
	}
}

// TestDocsSchemeReadDirOpenRoundTrip is a regression test for the editor
// completion / open path: every entry returned by ReadDir of the workspace
// root must either be a directory or be openable by name with non-empty
// content. Before memoryScheme respected directories, nested files (e.g.
// "develop/sdk.md") surfaced at the root by basename, but the basename did
// not resolve to a real URI and the editor opened an empty buffer.
func TestDocsSchemeReadDirOpenRoundTrip(t *testing.T) {
	uri, err := workspaceapi.ParseURI("docs:///")
	require.NoError(t, err)

	s, err := newDocsSchemeFunc()(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	entries, err := s.ReadDir("/")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		f, err := s.Open("/" + name)
		require.NoError(t, err, "open %s", name)
		data, err := io.ReadAll(f)
		_ = f.Close()
		require.NoError(t, err, "read %s", name)
		assert.NotEmpty(t, data, "%s should not be empty when opened by ReadDir-returned name", name)
	}
}

// TestDocsSchemeNestedDirsAreBrowsable asserts that the docs scheme exposes
// the embedded nested layout (develop/, learn/) as real directories, and that
// descending into those directories yields the markdown files stored under
// them.
func TestDocsSchemeNestedDirsAreBrowsable(t *testing.T) {
	uri, err := workspaceapi.ParseURI("docs:///")
	require.NoError(t, err)

	s, err := newDocsSchemeFunc()(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	root, err := s.ReadDir("/")
	require.NoError(t, err)

	sub := map[string]bool{}
	for _, e := range root {
		if e.IsDir() {
			sub[e.Name()] = true
		}
	}
	for _, want := range []string{"develop", "learn"} {
		assert.Truef(t, sub[want], "expected docs:/// to expose %q as a subdirectory", want)
	}

	develop, err := s.ReadDir("/develop")
	require.NoError(t, err)
	developFiles := map[string]bool{}
	for _, e := range develop {
		developFiles[e.Name()] = e.IsDir()
	}
	assert.Contains(t, developFiles, "sdk.md", "/develop must list sdk.md")
	assert.False(t, developFiles["sdk.md"], "/develop/sdk.md must be a file")

	f, err := s.Open("/develop/sdk.md")
	require.NoError(t, err)
	data, err := io.ReadAll(f)
	_ = f.Close()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func fsWalkMD(t *testing.T, out *map[string][]byte) error {
	t.Helper()
	entries, err := docsFS.ReadDir(docsSchemeRoot)
	if err != nil {
		return err
	}
	var walk func(dir string) error
	walk = func(dir string) error {
		es, err := docsFS.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range es {
			p := dir + "/" + e.Name()
			if e.IsDir() {
				if err := walk(p); err != nil {
					return err
				}
				continue
			}
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := docsFS.ReadFile(p)
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(p, docsSchemeRoot+"/")
			(*out)[rel] = data
		}
		return nil
	}
	_ = entries
	return walk(docsSchemeRoot)
}
