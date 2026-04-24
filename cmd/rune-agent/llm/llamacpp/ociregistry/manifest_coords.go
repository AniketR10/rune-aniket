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


package ociregistry

import (
	"path/filepath"
	"strings"
)

// manifestCoords parses the directory layout written by Cache.PutManifest
// (manifests/<host>/<repo path...>/<target>) back into its (host, repo,
// target) triple. Returns ok=false for paths that do not match the layout.
//
// Kept as an unexported helper so internal callers (Cache, tests) share a
// single parser; the CacheRegistry that previously owned this function
// has been folded into the llamacpp package which re-implements the
// mapping on top of the public Cache API.
func manifestCoords(root, path string) (host, repo, target string, ok bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", "", "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 {
		return "", "", "", false
	}
	host = parts[0]
	target = parts[len(parts)-1]
	repo = strings.Join(parts[1:len(parts)-1], "/")
	// Cache.ManifestPath rewrites "sha256:..." to "sha256-..." so the file
	// name is safe on Windows. Invert that here so digests round-trip to
	// their canonical form for anyone feeding the name back into Pull/Get.
	if alg, rest, cut := strings.Cut(target, "-"); cut && (alg == "sha256" || alg == "sha512") {
		target = alg + ":" + rest
	}
	return host, repo, target, true
}
