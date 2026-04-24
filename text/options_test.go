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
	"unstable.build/go-tui/cell"
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

func TestIndentRuneForURI(t *testing.T) {
	t.Parallel()

	indents := IndentConfig{
		"yaml": IndentRuneSpace,
		"go":   IndentRuneTab,
	}

	uri, err := workspaceapi.ParseURI("memory:///foo.yaml")
	require.NoError(t, err)
	r, ok := IndentRuneForURI(uri, nil, indents)
	require.True(t, ok)
	assert.Equal(t, IndentRuneSpace, r)
}

func newBufferWithContent(t *testing.T, content string) *cell.Buffer {
	t.Helper()
	buf := new(cell.Buffer)
	buf.Init()
	buf.WriteString(content)
	return buf
}

func TestIndentRuneForURI_Detection(t *testing.T) {
	t.Parallel()

	knownURI, err := workspaceapi.ParseURI("memory:///foo.py")
	require.NoError(t, err)
	unknownURI, err := workspaceapi.ParseURI("memory:///Makefilelike")
	require.NoError(t, err)

	tabsCfg := IndentConfig{"python": IndentRuneTab}
	spacesCfg := IndentConfig{"python": IndentRuneSpace}
	emptyCfg := IndentConfig{}

	tests := []struct {
		name    string
		uri     workspaceapi.URI
		content string
		useBuf  bool
		indents IndentConfig
		wantR   rune
		wantOK  bool
	}{
		{
			name:    "unknown language with empty buffer",
			uri:     unknownURI,
			useBuf:  true,
			indents: tabsCfg,
			wantOK:  false,
		},
		{
			name:    "unknown language with tab-indented buffer",
			uri:     unknownURI,
			content: "\tfoo\n",
			useBuf:  true,
			indents: tabsCfg,
			wantOK:  false,
		},
		{
			name:    "known language, no config, empty buffer",
			uri:     knownURI,
			useBuf:  true,
			indents: emptyCfg,
			wantOK:  false,
		},
		{
			name:    "known language, configured tabs, empty buffer",
			uri:     knownURI,
			useBuf:  true,
			indents: tabsCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
		{
			name:    "known language, configured spaces, empty buffer",
			uri:     knownURI,
			useBuf:  true,
			indents: spacesCfg,
			wantR:   IndentRuneSpace,
			wantOK:  true,
		},
		{
			name:    "configured tabs but buffer uses spaces",
			uri:     knownURI,
			content: "def foo():\n    return 1\n",
			useBuf:  true,
			indents: tabsCfg,
			wantR:   IndentRuneSpace,
			wantOK:  true,
		},
		{
			name:    "configured spaces but buffer uses tabs",
			uri:     knownURI,
			content: "def foo():\n\treturn 1\n",
			useBuf:  true,
			indents: spacesCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
		{
			name:    "configured tabs and buffer uses tabs",
			uri:     knownURI,
			content: "def foo():\n\treturn 1\n",
			useBuf:  true,
			indents: tabsCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
		{
			name:    "configured spaces and buffer uses spaces",
			uri:     knownURI,
			content: "def foo():\n    return 1\n",
			useBuf:  true,
			indents: spacesCfg,
			wantR:   IndentRuneSpace,
			wantOK:  true,
		},
		{
			name:    "mixed indentation falls back to configured tabs",
			uri:     knownURI,
			content: "def foo():\n\treturn 1\ndef bar():\n    return 2\n",
			useBuf:  true,
			indents: tabsCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
		{
			name:    "no configured rune, detected spaces bridge the gap",
			uri:     knownURI,
			content: "def foo():\n    return 1\n",
			useBuf:  true,
			indents: emptyCfg,
			wantR:   IndentRuneSpace,
			wantOK:  true,
		},
		{
			name:    "no configured rune, detected tabs bridge the gap",
			uri:     knownURI,
			content: "def foo():\n\treturn 1\n",
			useBuf:  true,
			indents: emptyCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
		{
			name:    "nil buffer falls back to configured rune",
			uri:     knownURI,
			useBuf:  false,
			indents: spacesCfg,
			wantR:   IndentRuneSpace,
			wantOK:  true,
		},
		{
			name:    "nil buffer with no config",
			uri:     knownURI,
			useBuf:  false,
			indents: emptyCfg,
			wantOK:  false,
		},
		{
			name:    "non-indented lines are inconclusive",
			uri:     knownURI,
			content: "hello\nworld\n",
			useBuf:  true,
			indents: tabsCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
		{
			name:    "single leading space is ignored",
			uri:     knownURI,
			content: " a leading space line\nanother\n",
			useBuf:  true,
			indents: tabsCfg,
			wantR:   IndentRuneTab,
			wantOK:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf *cell.Buffer
			if tc.useBuf {
				buf = newBufferWithContent(t, tc.content)
			}
			r, ok := IndentRuneForURI(tc.uri, buf, tc.indents)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantR, r)
			}
		})
	}
}

func TestDetectIndentRune(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		buf    *cell.Buffer
		wantR  rune
		wantOK bool
	}{
		{
			name:   "nil buffer",
			buf:    nil,
			wantOK: false,
		},
		{
			name:   "empty buffer",
			buf:    newBufferWithContent(t, ""),
			wantOK: false,
		},
		{
			name:   "blank lines only",
			buf:    newBufferWithContent(t, "\n\n\n"),
			wantOK: false,
		},
		{
			name:   "non-indented content",
			buf:    newBufferWithContent(t, "hello\nworld\n"),
			wantOK: false,
		},
		{
			name:   "tabs only",
			buf:    newBufferWithContent(t, "def foo():\n\treturn 1\n\treturn 2\n"),
			wantR:  IndentRuneTab,
			wantOK: true,
		},
		{
			name:   "spaces only",
			buf:    newBufferWithContent(t, "def foo():\n    return 1\n    return 2\n"),
			wantR:  IndentRuneSpace,
			wantOK: true,
		},
		{
			name:   "mixed tabs and spaces",
			buf:    newBufferWithContent(t, "def foo():\n\treturn 1\ndef bar():\n    return 2\n"),
			wantOK: false,
		},
		{
			name:   "single leading space is ignored",
			buf:    newBufferWithContent(t, " x\n"),
			wantOK: false,
		},
		{
			name:   "two or more leading spaces counts as spaces",
			buf:    newBufferWithContent(t, "  x\n"),
			wantR:  IndentRuneSpace,
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, ok := detectIndentRune(tc.buf)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantR, r)
			}
		})
	}
}
