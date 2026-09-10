// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package lspcmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type captureNotify struct {
	mu       sync.Mutex
	notifies []struct {
		level browserapi.NotificationLevel
		msg   string
	}
}

func (n *captureNotify) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifies = append(n.notifies, struct {
		level browserapi.NotificationLevel
		msg   string
	}{level: level, msg: fmt.Sprintf(msg, args...)})
	return "", nil
}

func (n *captureNotify) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return n.Notify(level, msg, args...)
}

func (n *captureNotify) UpdateNotificationProgress(_, _ string, _, _ int64) error {
	return nil
}

func (n *captureNotify) errors() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []string
	for _, e := range n.notifies {
		if e.level == browserapi.LevelError {
			out = append(out, e.msg)
		}
	}
	return out
}

func TestLSPHandlersSurfaceEmptyResult(t *testing.T) {
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)
	srcURI, err := workspaceapi.ParseURI("file:///project/a.go")
	require.NoError(t, err)

	empty := semanticapi.LocationResult{}
	lsp := &mockLSP{
		definitionFn: func(_ context.Context, _ semanticapi.DefinitionParams) (semanticapi.LocationResult, error) {
			return empty, nil
		},
		declarationFn: func(_ context.Context, _ semanticapi.DeclarationParams) (semanticapi.LocationResult, error) {
			return empty, nil
		},
		typeDefinitionFn: func(_ context.Context, _ semanticapi.TypeDefinitionParams) (semanticapi.LocationResult, error) {
			return empty, nil
		},
		implementationFn: func(_ context.Context, _ semanticapi.ImplementationParams) (semanticapi.LocationResult, error) {
			return empty, nil
		},
	}
	editor := &mockEditor{
		editorFn: func(u workspaceapi.URI) (textapi.Handler, error) {
			return &mockHandler{uri: u}, nil
		},
	}
	wm := &mockWindowManager{}
	parser := &mockParser{
		searchFn: func(query string, _ []string) (iterator.Iterator[syntaxapi.Result], error) {
			if strings.Contains(query, "qualified_type") {
				return iterator.FromSlice([]syntaxapi.Result{
					{File: srcURI, Text: "mylib", From: term.Coordinates{X: 1, Y: 10}, CaptureName: "pkg"},
					{File: srcURI, Text: "MyType", From: term.Coordinates{X: 7, Y: 10}, CaptureName: "type"},
				}), nil
			}
			return iterator.Empty[syntaxapi.Result](), nil
		},
		resolveFn: func(name string, _ syntaxapi.Progress) ([]syntaxapi.Match, error) {
			if name == "mylib.MyType" {
				return []syntaxapi.Match{
					{URI: srcURI.String(), Pos: term.Coordinates{X: 7, Y: 10}},
				}, nil
			}
			return nil, nil
		},
	}
	cmd := textapi.Command{
		URI:  srcURI,
		Args: []string{"mylib.MyType"},
	}

	cases := []struct {
		name string
		run  func(notify browserapi.Notifications) error
		want string
	}{
		{
			name: "definition",
			run: func(notify browserapi.Notifications) error {
				h := DefinitionHandler(
					lsp, editor, wm, &mockResourceOpener{}, notify, &mockFileSystem{},
					rootURI, syncTick, parser, DefaultDefinitionConfig(), nil,
				)
				return h.HandleCommand(context.Background(), cmd)
			},
			want: "definition",
		},
		{
			name: "declaration",
			run: func(notify browserapi.Notifications) error {
				h := DeclarationHandler(
					lsp, editor, wm, &mockResourceOpener{}, notify, &mockFileSystem{},
					rootURI, syncTick, parser, DefaultDeclarationConfig(), nil,
				)
				return h.HandleCommand(context.Background(), cmd)
			},
			want: "declaration",
		},
		{
			name: "type-definition",
			run: func(notify browserapi.Notifications) error {
				h := TypeDefinitionHandler(
					lsp, editor, wm, &mockResourceOpener{}, notify, &mockFileSystem{},
					rootURI, syncTick, parser, DefaultTypeDefinitionConfig(), nil,
				)
				return h.HandleCommand(context.Background(), cmd)
			},
			want: "type-definition",
		},
		{
			name: "implementation",
			run: func(notify browserapi.Notifications) error {
				h := ImplementationHandler(
					lsp, editor, wm, &mockResourceOpener{}, notify, &mockFileSystem{},
					rootURI, syncTick, parser, DefaultImplementationConfig(), nil,
				)
				return h.HandleCommand(context.Background(), cmd)
			},
			want: "implementation",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notify := &captureNotify{}
			err := tc.run(notify)
			require.NoError(t, err)
			var errs []string
			require.Eventually(t, func() bool {
				errs = notify.errors()
				return len(errs) > 0
			}, 2*time.Second, 5*time.Millisecond,
				"%s handler must surface an error notification when gopls "+
					"returns a success response with zero locations", tc.name)
			assert.Contains(t, strings.ToLower(errs[0]), tc.want,
				"error notification should mention the handler name")
		})
	}
}
