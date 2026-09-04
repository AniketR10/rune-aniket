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

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    Config
		wantErr bool
	}{
		{
			name: "valid stdio server",
			data: `{
				"mcpServers": {
					"rune": {
						"type": "stdio",
						"command": "runectl",
						"args": ["mcp"],
						"env": {"FOO": "bar"}
					}
				}
			}`,
			want: Config{
				MCPServers: map[string]ServerConfig{
					"rune": {
						Type:    "stdio",
						Command: "runectl",
						Args:    []string{"mcp"},
						Env:     map[string]string{"FOO": "bar"},
					},
				},
			},
		},
		{
			name: "multiple servers",
			data: `{
				"mcpServers": {
					"a": {"type": "stdio", "command": "a-cmd"},
					"b": {"type": "stdio", "command": "b-cmd", "args": ["--flag"]}
				}
			}`,
			want: Config{
				MCPServers: map[string]ServerConfig{
					"a": {Type: "stdio", Command: "a-cmd"},
					"b": {Type: "stdio", Command: "b-cmd", Args: []string{"--flag"}},
				},
			},
		},
		{
			name: "empty servers",
			data: `{"mcpServers": {}}`,
			want: Config{MCPServers: map[string]ServerConfig{}},
		},
		{
			name: "empty object",
			data: `{}`,
			want: Config{},
		},
		{
			name:    "malformed JSON",
			data:    `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig([]byte(tt.data))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
