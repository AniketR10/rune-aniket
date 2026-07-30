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
