// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package ide

import (
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
