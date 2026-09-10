// Copyright (C) 2017-2026 The Rune Authors
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

package runenet

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
)

func TestFromConfig(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	tsuite := []struct {
		name string
		cfg  map[string]interface{}
		want Config
	}{
		{
			name: "defaults",
			cfg:  map[string]interface{}{},
			want: Config{Hostname: hostname, Port: DefaultPort, Dir: "/data"},
		},
		{
			name: "explicit values",
			cfg: map[string]interface{}{
				"auto_join": true,
				"hostname":  "workstation",
				"port":      9999,
			},
			want: Config{
				AutoJoin: true,
				Hostname: "workstation",
				Port:     9999,
				Dir:      "/data",
			},
		},
		{
			// The coordination server and the pre-auth key are minted
			// by the account server, so a config that names them is
			// ignored rather than honoured.
			name: "coordination server is not configurable",
			cfg: map[string]interface{}{
				"control_url": "https://headscale.example.com",
				"auth_key":    "tskey-auth-secret",
			},
			want: Config{Hostname: hostname, Port: DefaultPort, Dir: "/data"},
		},
		{
			name: "zero port falls back to the default",
			cfg:  map[string]interface{}{"port": 0},
			want: Config{Hostname: hostname, Port: DefaultPort, Dir: "/data"},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			got, err := FromConfig(config.MapConfig(tcase.cfg), "/data")
			require.NoError(t, err)
			assert.Equal(t, tcase.want, got)
		})
	}
}
