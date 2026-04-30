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

package walkdir

import "context"

// Filter decides whether an entry produced by walkdir should be excluded.
// MatchRelPath returns true when the entry at the given workspace-relative
// path must be skipped. The signature deliberately takes a relative path
// (rather than a workspaceapi.URI) so walkdir can avoid an extra w.URI()
// round trip per directory entry — non-trivial on remote workspaces. The
// interface mirrors vctrl.Matcher.MatchRelPath so a vctrl.Matcher value
// satisfies Filter directly.
type Filter interface {
	MatchRelPath(relpath string, isDir bool) bool
}

type filterContextKey struct{}

// WithContextFilter returns a context configured to exclude entries matching f
// from walkdir traversal results. The filter is consulted for every regular
// file and every directory before recursing into it. Constructing a filter
// (e.g. via vctrl.LoadGitignore) is expensive, so callers should build the
// filter once and reuse the resulting context across walkdir invocations.
// Passing a nil filter is a no-op.
func WithContextFilter(ctx context.Context, f Filter) context.Context {
	if f == nil {
		return ctx
	}
	return context.WithValue(ctx, filterContextKey{}, f)
}

func filterFromContext(ctx context.Context) Filter {
	f, _ := ctx.Value(filterContextKey{}).(Filter)
	return f
}
