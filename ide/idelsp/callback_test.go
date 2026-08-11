// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
