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

package llmarg_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"

	"unstable.build/go-tui/cmd/rune-agent/llm/llmarg"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

// strictService wraps llmtest.Service so GetModel honours Provider,
// matching the host llmrouter contract; the default name-only
// llmtest.Service would hide the bug this package guards against.
type strictService struct {
	*llmtest.Service
	models []llmapi.ModelEntry
}

func (s *strictService) GetModel(
	_ context.Context, m llmapi.ModelEntry,
) (llmapi.ModelEntry, error) {
	if m.Provider == "" {
		return llmapi.ModelEntry{}, llmapi.ErrModelNotFound
	}
	for _, e := range s.models {
		if e.Name == m.Name && e.Provider == m.Provider {
			return e, nil
		}
	}
	return llmapi.ModelEntry{}, llmapi.ErrModelNotFound
}

func newStrictService(models ...llmapi.ModelEntry) *strictService {
	return &strictService{
		Service: llmtest.New(models),
		models:  models,
	}
}

func TestResolve(t *testing.T) {
	svc := newStrictService(
		llmapi.ModelEntry{Provider: "openai", Name: "gpt-5.5"},
		llmapi.ModelEntry{Provider: "codex", Name: "gpt-5.5"},
		llmapi.ModelEntry{Provider: "openai", Name: "gpt-4o"},
	)
	ctx := context.Background()

	type want struct {
		provider string
		errMatch string
	}
	cases := []struct {
		name string
		arg  string
		want want
	}{
		{"qualified codex", "codex/gpt-5.5", want{provider: "codex"}},
		{"qualified openai", "openai/gpt-5.5", want{provider: "openai"}},
		{"bare unique", "gpt-4o", want{provider: "openai"}},
		{
			"bare ambiguous",
			"gpt-5.5",
			want{errMatch: `ambiguous: prefix with the intended provider (also available as codex/gpt-5.5, openai/gpt-5.5)`},
		},
		{"bogus provider", "bogus/gpt-5.5", want{errMatch: `is not available`}},
		{"unknown qualified", "codex/does-not-exist", want{errMatch: `is not available`}},
		{"bare unknown", "nothing", want{errMatch: `is not available`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := llmarg.Resolve(ctx, svc, tc.arg)
			if tc.want.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.want.errMatch)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want.provider, entry.Provider)
		})
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		prov string
		name string
	}{
		{"openai/gpt-5.5", true, "openai", "gpt-5.5"},
		{"codex/gpt-5.5", true, "codex", "gpt-5.5"},
		{"gpt-5.5", false, "", ""},
		{"/gpt-5.5", false, "", ""},
		{"openai/", false, "", ""},
		{"", false, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			p, n, ok := llmarg.Split(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.prov, p)
			assert.Equal(t, tc.name, n)
		})
	}
}

func TestQualify(t *testing.T) {
	cases := []struct {
		name  string
		entry llmapi.ModelEntry
		want  string
	}{
		{
			name:  "provider and name",
			entry: llmapi.ModelEntry{Provider: "anthropic", Name: "claude-opus-4-8"},
			want:  "anthropic/claude-opus-4-8",
		},
		{
			name:  "name only",
			entry: llmapi.ModelEntry{Name: "gpt-5.5"},
			want:  "gpt-5.5",
		},
		{
			name:  "empty",
			entry: llmapi.ModelEntry{},
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := llmarg.Qualify(tc.entry)
			assert.Equal(t, tc.want, got)
			if p, n, ok := llmarg.Split(got); ok {
				assert.Equal(t, tc.entry.Provider, p)
				assert.Equal(t, tc.entry.Name, n)
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	svc := newStrictService(
		llmapi.ModelEntry{Provider: "openai", Name: "gpt-5.5"},
		llmapi.ModelEntry{Provider: "codex", Name: "gpt-5.5"},
		llmapi.ModelEntry{Provider: "openai", Name: "gpt-4o"},
	)
	got := llmarg.Available(context.Background(), svc)
	assert.Equal(t, "codex/gpt-5.5, openai/gpt-4o, openai/gpt-5.5", got)
}

// aliasService resolves the bare name "default" through GetModel,
// mirroring how the host router exposes alias resolution. Any other
// empty-provider name returns ErrModelNotFound.
type aliasService struct {
	*llmtest.Service
	target llmapi.ModelEntry
}

func (s *aliasService) GetModel(
	_ context.Context, m llmapi.ModelEntry,
) (llmapi.ModelEntry, error) {
	if m.Provider == "" && m.Name == "default" {
		return s.target, nil
	}
	return llmapi.ModelEntry{}, llmapi.ErrModelNotFound
}

// TestResolve_BareAliasResolvesViaGetModel verifies a bare name that the
// service resolves through GetModel (an alias) is returned directly,
// without requiring a single-provider catalog match.
func TestResolve_BareAliasResolvesViaGetModel(t *testing.T) {
	target := llmapi.ModelEntry{Provider: "openai", Name: "gpt-5.5"}
	svc := &aliasService{
		Service: llmtest.New([]llmapi.ModelEntry{target}),
		target:  target,
	}
	entry, err := llmarg.Resolve(context.Background(), svc, "default")
	require.NoError(t, err)
	assert.Equal(t, target, entry)
}
