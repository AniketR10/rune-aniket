// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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
