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


package extutil

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/text/standard"
	"unstable.build/go-tui/text/vi"
)

func TestEditor(t *testing.T) {
	modalType := reflect.TypeOf(vi.Editor())
	modelessType := reflect.TypeOf(standard.Editor())

	tests := []struct {
		name string
		cfg  map[string]any
		want reflect.Type
	}{
		{
			name: "no editor config",
			cfg:  map[string]any{},
			want: modalType,
		},
		{
			name: "editor config without mode defaults to modal",
			cfg:  map[string]any{"editor": map[string]any{}},
			want: modalType,
		},
		{
			name: "modal",
			cfg:  map[string]any{"editor": map[string]any{"mode": "modal"}},
			want: modalType,
		},
		{
			name: "modeless",
			cfg:  map[string]any{"editor": map[string]any{"mode": "modeless"}},
			want: modelessType,
		},
		{
			name: "exo no fallback defaults to modeless",
			cfg:  map[string]any{"editor": map[string]any{"mode": "exo"}},
			want: modelessType,
		},
		{
			name: "exo fallback modal",
			cfg: map[string]any{"editor": map[string]any{
				"mode": "exo",
				"exo":  map[string]any{"fallback": "modal"},
			}},
			want: modalType,
		},
		{
			name: "exo fallback modeless",
			cfg: map[string]any{"editor": map[string]any{
				"mode": "exo",
				"exo":  map[string]any{"fallback": "modeless"},
			}},
			want: modelessType,
		},
		{
			name: "exo unknown fallback defaults to modeless",
			cfg: map[string]any{"editor": map[string]any{
				"mode": "exo",
				"exo":  map[string]any{"fallback": "bogus"},
			}},
			want: modelessType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ed, err := Editor(clipboard.NewInMemory(), config.MapConfig(tc.cfg))
			require.NoError(t, err)
			require.Equal(t, tc.want, reflect.TypeOf(ed))
		})
	}
}

func TestEditorModal(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
		want bool
	}{
		{"no editor config", map[string]any{}, true},
		{"empty editor config defaults to modal", map[string]any{"editor": map[string]any{}}, true},
		{"modal", map[string]any{"editor": map[string]any{"mode": "modal"}}, true},
		{"modeless", map[string]any{"editor": map[string]any{"mode": "modeless"}}, false},
		{"exo no fallback defaults to modeless", map[string]any{"editor": map[string]any{"mode": "exo"}}, false},
		{
			name: "exo fallback modal",
			cfg: map[string]any{"editor": map[string]any{
				"mode": "exo",
				"exo":  map[string]any{"fallback": "modal"},
			}},
			want: true,
		},
		{
			name: "exo fallback modeless",
			cfg: map[string]any{"editor": map[string]any{
				"mode": "exo",
				"exo":  map[string]any{"fallback": "modeless"},
			}},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modal, err := EditorModal(config.MapConfig(tc.cfg))
			require.NoError(t, err)
			require.Equal(t, tc.want, modal)
		})
	}
}
