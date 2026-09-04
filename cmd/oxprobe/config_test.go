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

package main

import (
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildProbesIncludesPkgLatestWhenConfigured(t *testing.T) {
	cfg := environments["prod"]

	layers := probeLayers(buildProbes(cfg, http.DefaultClient, false))

	require.Contains(t, layers, "pkg_latest")
}

func TestBuildProbesOmitsPkgLatestWithoutArchs(t *testing.T) {
	cfg := environments["prod"]
	cfg.Archs = nil

	layers := probeLayers(buildProbes(cfg, http.DefaultClient, false))

	require.NotContains(t, layers, "pkg_latest")
}

func TestPkgLatestPagesOnFailure(t *testing.T) {
	cfg := environments["prod"]

	for _, p := range buildProbes(cfg, http.DefaultClient, false) {
		if p.Layer() == "pkg_latest" {
			require.True(t, p.Critical())
			return
		}
	}
	t.Fatal("pkg_latest probe not built")
}

func probeLayers[T interface{ Layer() string }](probes []T) []string {
	layers := make([]string, 0, len(probes))
	for _, p := range probes {
		layers = append(layers, p.Layer())
	}
	slices.Sort(layers)
	return layers
}
