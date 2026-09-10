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

package idelsp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

func TestCallbackAdapterRequestErrorHasNilResult(t *testing.T) {
	t.Parallel()

	callbackErr := errors.New("callback failed")
	adapter := &callbackAdapter{cb: &failingLSPCallback{err: callbackErr}}
	methods := []string{
		"window/showDocument",
		"window/showMessageRequest",
		"window/workDoneProgress/create",
		"workspace/applyEdit",
		"workspace/workspaceFolders",
		"workspace/configuration",
		"client/registerCapability",
		"client/unregisterCapability",
		"workspace/codeLens/refresh",
		"workspace/semanticTokens/refresh",
		"workspace/inlayHint/refresh",
		"workspace/diagnostic/refresh",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			result, err := adapter.handleRequest(
				context.Background(), method, json.RawMessage(`{}`),
			)
			require.ErrorIs(t, err, callbackErr)
			if result != nil {
				t.Fatalf("result = %#v, want nil", result)
			}
		})
	}
}

type failingLSPCallback struct {
	semanticapi.LSPCallback
	err error
}

func (c *failingLSPCallback) ShowDocument(
	context.Context, semanticapi.ShowDocumentParams,
) (semanticapi.ShowDocumentResult, error) {
	return semanticapi.ShowDocumentResult{}, c.err
}

func (c *failingLSPCallback) ShowMessageRequest(
	context.Context, semanticapi.ShowMessageRequestParams,
) (*semanticapi.MessageActionItem, error) {
	return nil, c.err
}

func (c *failingLSPCallback) WorkDoneProgressCreate(
	context.Context, semanticapi.WorkDoneProgressCreateParams,
) error {
	return c.err
}

func (c *failingLSPCallback) ApplyEdit(
	context.Context, semanticapi.ApplyWorkspaceEditParams,
) (semanticapi.ApplyWorkspaceEditResult, error) {
	return semanticapi.ApplyWorkspaceEditResult{}, c.err
}

func (c *failingLSPCallback) WorkspaceFolders(
	context.Context,
) ([]semanticapi.WorkspaceFolder, error) {
	return nil, c.err
}

func (c *failingLSPCallback) Configuration(
	context.Context, semanticapi.ConfigurationParams,
) ([]json.RawMessage, error) {
	return nil, c.err
}

func (c *failingLSPCallback) RegisterCapability(
	context.Context, semanticapi.RegistrationParams,
) error {
	return c.err
}

func (c *failingLSPCallback) UnregisterCapability(
	context.Context, semanticapi.UnregistrationParams,
) error {
	return c.err
}

func (c *failingLSPCallback) CodeLensRefresh(context.Context) error {
	return c.err
}

func (c *failingLSPCallback) SemanticTokensRefresh(context.Context) error {
	return c.err
}

func (c *failingLSPCallback) InlayHintRefresh(context.Context) error {
	return c.err
}

func (c *failingLSPCallback) DiagnosticRefresh(context.Context) error {
	return c.err
}
