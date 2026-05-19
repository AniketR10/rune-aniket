// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package ide

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// workspaceBasename returns the sanitised basename of the workspace
// path so it can be embedded in shell arguments without surprises.
// Works for every scheme: commands run via the workspace's executor in
// the workspace's own filesystem, so the URI path is a meaningful
// identifier whether the workspace is local, remote, or in-memory.
func workspaceBasename(uri workspaceapi.URI) string {
	base := filepath.Base(filepath.Clean(uri.Path()))
	return sanitiseBasename(base)
}

// workspaceHash returns a short, stable digest derived from the
// workspace URI. Two workspaces with the same basename but different
// underlying URIs produce different hashes so derived names (for
// example `$WORKSPACE-$WORKSPACE_HASH`) do not collide. The hash
// includes the scheme and host so the same path served by different
// hosts is differentiated.
func workspaceHash(uri workspaceapi.URI) string {
	key := uri.Scheme() + "://" + uri.Host() + filepath.Clean(uri.Path())
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:2])
}

// sanitiseBasename replaces characters that are awkward in shell
// arguments / on-disk path segments with '_'. The replacement set is
// deliberately conservative — anything that is not an ASCII letter,
// digit, '.', '-', or '_' is replaced.
func sanitiseBasename(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
