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

package sandbox

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// stubParser answers every syntax query with an empty result set.
type stubParser struct{}

var _ syntaxapi.Parser = stubParser{}

func (stubParser) Search(string, []string, ...string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (stubParser) SearchNode(syntaxapi.NodeCaptureName, ...string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (stubParser) Query(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (stubParser) QueryNode(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (stubParser) Highlight(workspaceapi.URI, string) (iterator.Iterator[textapi.Location], error) {
	return iterator.Empty[textapi.Location](), nil
}

func (stubParser) ResolveSymbol(context.Context, string, syntaxapi.Progress) (iterator.Iterator[syntaxapi.Match], error) {
	return iterator.Empty[syntaxapi.Match](), nil
}

func (stubParser) ListReferencedSymbols(context.Context) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

// stubLLM exposes no models; calls fail with the canonical service
// errors so extensions can exercise their error paths.
type stubLLM struct{}

var _ llmapi.Service = stubLLM{}

func (stubLLM) CreateCompletion(
	context.Context, llmapi.ModelEntry, llmapi.Request,
) (iterator.Iterator[llmapi.Event], error) {
	return iterator.Empty[llmapi.Event](), nil
}

func (stubLLM) CountTokens(llmapi.ModelEntry, []llmapi.Message) (int, error) {
	return 0, nil
}

func (stubLLM) Models() iterator.Iterator[llmapi.ModelEntry] {
	return iterator.Empty[llmapi.ModelEntry]()
}

func (stubLLM) GetModel(context.Context, llmapi.ModelEntry) (llmapi.ModelEntry, error) {
	return llmapi.ModelEntry{}, llmapi.ErrModelNotFound
}
