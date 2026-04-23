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

package text

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestDefaultConfigPkgManagerReturnsNotFound(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	it, err := cfg.PkgManager.LibDir(context.Background(), "go")
	require.Nil(t, it)
	require.ErrorIs(t, err, storageapi.ErrNotFound)
}

func TestWithComments(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	assert.Empty(t, cfg.Comments)

	comments := CommentConfig{
		"go": {
			Line:  []string{"//"},
			Block: []CommentBlock{{Start: "/*", End: "*/"}},
		},
	}
	WithComments(comments)(&cfg)
	assert.Equal(t, comments, cfg.Comments)

	uri, err := workspaceapi.ParseURI("memory:///foo.go")
	require.NoError(t, err)
	spec, ok := CommentSpecForURI(uri, cfg.Comments)
	require.True(t, ok)
	assert.True(t, spec.HasLine())
	assert.True(t, spec.HasBlock())
	assert.Equal(t, []CommentBlock{{Start: "/*", End: "*/"}}, spec.Block)
}

func TestCommentConfigForLanguageNil(t *testing.T) {
	t.Parallel()

	var cc CommentConfig
	spec, ok := cc.ForLanguage("go")
	assert.False(t, ok)
	assert.Equal(t, CommentSpec{}, spec)
}

func TestCommentSpecForURIUnknownLanguage(t *testing.T) {
	t.Parallel()

	comments := CommentConfig{"go": {Line: []string{"//"}}}
	// A file without an extension cannot be resolved to a language ID.
	uri, err := workspaceapi.ParseURI("memory:///Makefilelike")
	require.NoError(t, err)
	spec, ok := CommentSpecForURI(uri, comments)
	assert.False(t, ok)
	assert.Equal(t, CommentSpec{}, spec)
}
