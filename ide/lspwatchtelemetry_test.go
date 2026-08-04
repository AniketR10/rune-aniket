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

package ide

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// stubWatchLSP embeds the interface so only the methods the test exercises
// need an implementation; any other call is a test bug and nil-panics.
type stubWatchLSP struct {
	semanticapi.LSP
	watched     []semanticapi.DidChangeWatchedFilesParams
	definitions int
}

func (s *stubWatchLSP) DidChangeWatchedFiles(
	_ context.Context, params semanticapi.DidChangeWatchedFilesParams,
) error {
	s.watched = append(s.watched, params)
	return nil
}

func (s *stubWatchLSP) Definition(
	_ context.Context, _ semanticapi.DefinitionParams,
) (semanticapi.LocationResult, error) {
	s.definitions++
	return semanticapi.LocationResult{}, nil
}

func TestWatchedFilesTelemetryLSP(t *testing.T) {
	stub := new(stubWatchLSP)
	var counts []int
	lsp := semanticapi.LSP(watchedFilesTelemetryLSP{
		LSP:      stub,
		onChange: func(n int) { counts = append(counts, n) },
	})

	params := semanticapi.DidChangeWatchedFilesParams{
		Changes: []semanticapi.FileEvent{
			{URI: "file:///a.go", Type: semanticapi.FileChangeTypeChanged},
			{URI: "file:///b.go", Type: semanticapi.FileChangeTypeCreated},
		},
	}
	require.NoError(t, lsp.DidChangeWatchedFiles(context.Background(), params))
	assert.Equal(t, []int{2}, counts)
	assert.Equal(t, []semanticapi.DidChangeWatchedFilesParams{params}, stub.watched)

	_, err := lsp.Definition(context.Background(), semanticapi.DefinitionParams{})
	require.NoError(t, err)
	assert.Equal(t, 1, stub.definitions)
	assert.Equal(t, []int{2}, counts)
}
