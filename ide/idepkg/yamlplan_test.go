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

package idepkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
	"gopkg.in/yaml.v3"
)

func TestPlanConfigChange(t *testing.T) {
	t.Parallel()

	const (
		pkgID      = "testpkg"
		dataDir    = "/data"
		editorMode = "vim"
	)

	tests := []struct {
		name       string
		pkgConfig  string
		userCfg    map[string]any
		version    release.Version
		wantPrompt bool
		wantDiff   map[string]any
		wantErr    bool
	}{
		{
			name:       "empty overlay yields no prompt",
			pkgConfig:  "{}\n",
			userCfg:    map[string]any{},
			version:    "1",
			wantPrompt: false,
		},
		{
			name:       "all keys present, static and equal, no prompt",
			pkgConfig:  "a: 1\nb: hello\n",
			userCfg:    map[string]any{"a": 1, "b": "hello"},
			version:    "1",
			wantPrompt: false,
		},
		{
			name:       "new top-level key prompts with the new key",
			pkgConfig:  "a: 1\nc: 3\n",
			userCfg:    map[string]any{"a": 1},
			version:    "1",
			wantPrompt: true,
			wantDiff:   map[string]any{"c": 3},
		},
		{
			name:       "new nested sub-key only prompts with nested diff",
			pkgConfig:  "env:\n  A: 1\n  B: 2\n",
			userCfg:    map[string]any{"env": map[string]any{"A": 1}},
			version:    "1",
			wantPrompt: true,
			wantDiff:   map[string]any{"env": map[string]any{"B": 2}},
		},
		{
			name:       "static differing scalar present on both, no prompt (RUNE-187)",
			pkgConfig:  "a: package\n",
			userCfg:    map[string]any{"a": "user"},
			version:    "1",
			wantPrompt: false,
		},
		{
			name:       "version-dependent scalar changed value prompts (RUNE-225)",
			pkgConfig:  "goroot: /data/pkg/testpkg/$RUNE_PKG_VERSION/go\n",
			userCfg:    map[string]any{"goroot": "/data/pkg/testpkg/1/go"},
			version:    "2",
			wantPrompt: true,
			wantDiff:   map[string]any{"goroot": "/data/pkg/testpkg/2/go"},
		},
		{
			name:       "version-dependent scalar unchanged value, no prompt",
			pkgConfig:  "goroot: /data/pkg/testpkg/$RUNE_PKG_VERSION/go\n",
			userCfg:    map[string]any{"goroot": "/data/pkg/testpkg/1/go"},
			version:    "1",
			wantPrompt: false,
		},
		{
			name: "version-dependent and static nested mixed, only vdep and new keys in diff",
			pkgConfig: "env:\n" +
				"  GOROOT: /data/pkg/testpkg/$RUNE_PKG_VERSION/go\n" +
				"  STATIC: package\n" +
				"  NEW: added\n",
			userCfg: map[string]any{"env": map[string]any{
				"GOROOT": "/data/pkg/testpkg/1/go",
				"STATIC": "user",
			}},
			version:    "2",
			wantPrompt: true,
			wantDiff: map[string]any{"env": map[string]any{
				"GOROOT": "/data/pkg/testpkg/2/go",
				"NEW":    "added",
			}},
		},
		{
			name:       "brace form of RUNE_PKG_VERSION detected",
			pkgConfig:  "goroot: /data/pkg/testpkg/${RUNE_PKG_VERSION}/go\n",
			userCfg:    map[string]any{"goroot": "/data/pkg/testpkg/1/go"},
			version:    "2",
			wantPrompt: true,
			wantDiff:   map[string]any{"goroot": "/data/pkg/testpkg/2/go"},
		},
		{
			name:       "non-version var differing, no prompt",
			pkgConfig:  "datadir: $RUNE_DATADIR/cache\n",
			userCfg:    map[string]any{"datadir": "/old/cache"},
			version:    "1",
			wantPrompt: false,
		},
		{
			name:       "int/bool coercions compared via fmt.Sprint, no prompt",
			pkgConfig:  "count: 5\nenabled: true\n",
			userCfg:    map[string]any{"count": int64(5), "enabled": true},
			version:    "1",
			wantPrompt: false,
		},
		{
			name:      "malformed yaml returns error",
			pkgConfig: "a: [unterminated\n",
			userCfg:   map[string]any{},
			version:   "1",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan, err := planConfigChange(
				"config.yaml", []byte(tt.pkgConfig), tt.userCfg,
				pkgID, tt.version, dataDir, editorMode,
			)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrompt, plan.prompt)
			if !tt.wantPrompt {
				assert.Nil(t, plan.pkgDoc)
				assert.Nil(t, plan.userDoc)
				return
			}
			require.NotNil(t, plan.pkgDoc)
			require.NotNil(t, plan.userDoc)
			require.NotEmpty(t, plan.missingYAML)
			assert.Equal(t, normalizeYAML(t, tt.wantDiff), docToMap(t, plan.pkgDoc))
		})
	}
}

func TestBuildMergedConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		user     string
		pkg      string
		want     map[string]any
		wantDiff map[string]any
	}{
		{
			name:     "append new key",
			user:     "a: 1\n",
			pkg:      "b: 2\n",
			want:     map[string]any{"a": 1, "b": 2},
			wantDiff: map[string]any{"b": 2},
		},
		{
			name:     "overwrite version-dependent key",
			user:     "goroot: /old\n",
			pkg:      "goroot: /new\n",
			want:     map[string]any{"goroot": "/new"},
			wantDiff: map[string]any{"goroot": "/new"},
		},
		{
			name:     "preserve untouched user keys",
			user:     "a: 1\nkeep: yes\n",
			pkg:      "b: 2\n",
			want:     map[string]any{"a": 1, "keep": "yes", "b": 2},
			wantDiff: map[string]any{"b": 2},
		},
		{
			name:     "nested merge adds sub-key without clobbering siblings",
			user:     "env:\n  A: 1\n",
			pkg:      "env:\n  B: 2\n",
			want:     map[string]any{"env": map[string]any{"A": 1, "B": 2}},
			wantDiff: map[string]any{"env": map[string]any{"B": 2}},
		},
	}

	for _, tt := range tests {
		t.Run("yaml/"+tt.name, func(t *testing.T) {
			t.Parallel()
			userDoc := mustParseYAML(t, tt.user)
			pkgDoc := mustParseYAML(t, tt.pkg)
			merged, err := buildMergedConfig(userDoc, pkgDoc, false)
			require.NoError(t, err)
			require.NotNil(t, merged.yamlDoc)
			assert.Nil(t, merged.starDiff)
			assert.Equal(t, normalizeYAML(t, tt.want), docToMap(t, merged.yamlDoc))
		})
		t.Run("star/"+tt.name, func(t *testing.T) {
			t.Parallel()
			userDoc := mustParseYAML(t, tt.user)
			pkgDoc := mustParseYAML(t, tt.pkg)
			merged, err := buildMergedConfig(userDoc, pkgDoc, true)
			require.NoError(t, err)
			require.NotNil(t, merged.starDiff)
			assert.Equal(t, normalizeYAML(t, tt.wantDiff), merged.starDiff)
		})
	}
}

func docToMap(t *testing.T, doc *yaml.Node) map[string]any {
	t.Helper()
	cfg, err := loadIdePkgConfigFromYAMLDoc(doc)
	require.NoError(t, err)
	return cfg
}

// normalizeYAML round-trips want through YAML so int/string scalar types
// match the decoded representation produced by docToMap.
func normalizeYAML(t *testing.T, want map[string]any) map[string]any {
	t.Helper()
	data, err := yaml.Marshal(want)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, yaml.Unmarshal(data, &out))
	return normalizeIdePkgConfig(out).(map[string]any)
}

// TestVersionDependentByDecode asserts that diffing a real-version decode
// against a sentinel-version decode marks exactly the leaves whose value
// changed, mirroring the nesting, so .star overlays re-prompt on version
// bumps identically to YAML (RUNE-225).
func TestVersionDependentByDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		real, sentinel map[string]any
		want           map[string]any
	}{
		{
			name:     "no difference",
			real:     map[string]any{"a": "x"},
			sentinel: map[string]any{"a": "x"},
			want:     nil,
		},
		{
			name:     "top-level scalar differs",
			real:     map[string]any{"a": "v1", "b": "same"},
			sentinel: map[string]any{"a": "v0", "b": "same"},
			want:     map[string]any{"a": true},
		},
		{
			name:     "nested scalar differs keeps sibling untouched",
			real:     map[string]any{"env": map[string]any{"GOROOT": "/1", "STATIC": "x"}},
			sentinel: map[string]any{"env": map[string]any{"GOROOT": "/0", "STATIC": "x"}},
			want:     map[string]any{"env": map[string]any{"GOROOT": true}},
		},
		{
			name:     "key missing in sentinel is ignored",
			real:     map[string]any{"a": "v1"},
			sentinel: map[string]any{},
			want:     nil,
		},
		{
			name:     "type change map vs scalar is not version-dependent",
			real:     map[string]any{"a": map[string]any{"b": 1}},
			sentinel: map[string]any{"a": "scalar"},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, versionDependentByDecode(tt.real, tt.sentinel))
		})
	}
}
