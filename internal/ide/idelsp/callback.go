// Copyright (C) 2017-2026 Unstable Build, LLC
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
	"fmt"
	"log/slog"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"unstable.build/rune/internal/ide/idelsp/jsonrpc2"
)

// callbackAdapter adapts semanticapi.LSPCallback to jsonrpc2.Handler.
type callbackAdapter struct {
	cb         semanticapi.LSPCallback
	serverName string
	rootURI    string
	log        *slog.Logger
}

// Compile-time assertion that callbackAdapter implements jsonrpc2.Handler.
var _ jsonrpc2.Handler = (*callbackAdapter)(nil)

func newCallbackAdapter(cb semanticapi.LSPCallback, serverName string, rootURI string) jsonrpc2.Handler {
	if cb == nil {
		return jsonrpc2.HandlerFunc(
			func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
				return nil, jsonrpc2.ErrNotHandled
			})
	}
	return &callbackAdapter{
		cb:         cb,
		serverName: serverName,
		rootURI:    rootURI,
		log: slog.With("struct", "idelsp.callbackAdapter",
			"server", serverName, "workspace", rootURI),
	}
}

// Handle implements jsonrpc2.Handler, processing server-initiated messages.
func (a *callbackAdapter) Handle(
	ctx context.Context, req *jsonrpc2.Request,
) (any, error) {
	ctx = ContextWithMetadata(ctx, Metadata{
		ServerName: a.serverName,
		RootURI:    a.rootURI,
	})
	// Notifications have no ID; handle and return nil.
	if !req.IsCall() {
		a.handleNotification(ctx, req.Method, req.Params)
		return nil, nil
	}
	// Requests have an ID; handle and return result.
	return a.handleRequest(ctx, req.Method, req.Params)
}

// handleNotification handles server-initiated notifications.
func (a *callbackAdapter) handleNotification(
	ctx context.Context, method string, params json.RawMessage,
) {
	var err error
	switch method {
	case "window/showMessage":
		var p semanticapi.ShowMessageParams
		err = json.Unmarshal(params, &p)
		if err == nil {
			err = a.cb.ShowMessage(ctx, p)
		}
	case "window/logMessage":
		var p semanticapi.LogMessageParams
		err = json.Unmarshal(params, &p)
		if err == nil {
			err = a.cb.LogMessage(ctx, p)
		}
	case "textDocument/publishDiagnostics":
		var p semanticapi.PublishDiagnosticsParams
		err = json.Unmarshal(params, &p)
		if err == nil {
			err = a.cb.PublishDiagnostics(ctx, p)
		}
	case "$/progress":
		var p semanticapi.ProgressParams
		err = json.Unmarshal(params, &p)
		if err == nil {
			err = a.cb.Progress(ctx, p)
		}
	case "$/logTrace":
		var p semanticapi.LogTraceParams
		err = json.Unmarshal(params, &p)
		if err == nil {
			err = a.cb.LogTrace(ctx, p)
		}
	default:
		// Route unrecognized notifications (e.g. rust-analyzer's
		// experimental/* extensions) to the generic inbound hook so
		// extensions can observe them.
		err = a.cb.HandleNotification(ctx, method, params)
	}
	if err != nil {
		a.log.Warn("idelsp: callback error", "method", method, "error", err)
	}
}

// handleRequest handles server-initiated requests.
func (a *callbackAdapter) handleRequest(
	ctx context.Context, method string, params json.RawMessage,
) (any, error) {
	switch method {
	case "window/showDocument":
		var p semanticapi.ShowDocumentParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(a.cb.ShowDocument(ctx, p))

	case "window/showMessageRequest":
		var p semanticapi.ShowMessageRequestParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(a.cb.ShowMessageRequest(ctx, p))

	case "window/workDoneProgress/create":
		var p semanticapi.WorkDoneProgressCreateParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(struct{}{}, a.cb.WorkDoneProgressCreate(ctx, p))

	case "workspace/applyEdit":
		var p semanticapi.ApplyWorkspaceEditParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(a.cb.ApplyEdit(ctx, p))

	case "workspace/workspaceFolders":
		return requestResult(a.cb.WorkspaceFolders(ctx))

	case "workspace/configuration":
		var p semanticapi.ConfigurationParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(a.cb.Configuration(ctx, p))

	case "client/registerCapability":
		var p semanticapi.RegistrationParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(struct{}{}, a.cb.RegisterCapability(ctx, p))

	case "client/unregisterCapability":
		var p semanticapi.UnregistrationParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return requestResult(struct{}{}, a.cb.UnregisterCapability(ctx, p))

	case "workspace/codeLens/refresh":
		return requestResult(struct{}{}, a.cb.CodeLensRefresh(ctx))

	case "workspace/semanticTokens/refresh":
		return requestResult(struct{}{}, a.cb.SemanticTokensRefresh(ctx))

	case "workspace/inlayHint/refresh":
		return requestResult(struct{}{}, a.cb.InlayHintRefresh(ctx))

	case "workspace/diagnostic/refresh":
		return requestResult(struct{}{}, a.cb.DiagnosticRefresh(ctx))

	default:
		return nil, fmt.Errorf("unknown request method: %s", method)
	}
}

func requestResult[T any](result T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return result, nil
}
