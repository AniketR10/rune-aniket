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

package symbolresolve

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/workspace/walkdir"
)

// registry lists the language specs in resolution-preference order. Go
// is tried first to preserve existing behavior.
var registry = []*Spec{Go, Python, Rust, Zig}

// SpecFor returns the spec for a tree-sitter language id, or nil when no
// spec is registered for that language.
func SpecFor(langID string) *Spec {
	for _, s := range registry {
		if s.LangID == langID {
			return s
		}
	}
	return nil
}

// SpecForFile returns the spec whose extensions match the given path or
// URI, or nil when no registered language claims it.
func SpecForFile(path string) *Spec {
	for _, s := range registry {
		if len(s.Extensions) > 0 && s.matchesFile(path) {
			return s
		}
	}
	return nil
}

// AllSpecs returns the registered language specs in
// resolution-preference order.
func AllSpecs() []*Spec {
	specs := make([]*Spec, len(registry))
	copy(specs, registry)
	return specs
}

// DetectSpecs walks the workspace lazily and yields each registered spec whose
// extensions match at least one present file. A spec is emitted as soon as the
// first matching file is seen, so a consumer can begin resolving against it
// before the walk completes. This is the single language-detection point for
// symbol resolution.
func DetectSpecs(ctx context.Context, fs walkdir.Reader) iterator.Iterator[Spec] {
	paths, err := walkdir.ListFiles(ctx, fs, ".")
	if err != nil {
		return iterator.Empty[Spec]()
	}

	pending := make([]*Spec, len(registry))
	copy(pending, registry)

	next := func(ctx context.Context) (Spec, bool, error) {
		for {
			file, ok := paths.Next(ctx)
			if !ok {
				return Spec{}, false, paths.Err()
			}
			for i, spec := range pending {
				if spec.matchesFile(file) {
					pending = append(pending[:i], pending[i+1:]...)
					return *spec, true, nil
				}
			}
		}
	}
	return iterator.FromFunc(next, paths.Close)
}
