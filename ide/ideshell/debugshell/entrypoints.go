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

package debugshell

import (
	"context"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/debug"
)

type entrypointQuery struct {
	query    string
	captures []string
	match    func(syntaxapi.Result) bool
}

var entrypointQueries = map[string]entrypointQuery{
	"go": {
		query:    `(function_declaration name: (identifier) @name)`,
		captures: []string{"name"},
		match:    func(r syntaxapi.Result) bool { return r.Text == "main" },
	},
	"rust": {
		query:    `(function_item name: (identifier) @name)`,
		captures: []string{"name"},
		match:    func(r syntaxapi.Result) bool { return r.Text == "main" },
	},
	"python": {
		query: `(if_statement
			condition: (comparison_operator
				(identifier) @name
				(string)))`,
		captures: []string{"name"},
		match:    func(r syntaxapi.Result) bool { return r.Text == "__name__" },
	},
}

func streamEntrypointPaths(
	parser syntaxapi.Parser, root workspaceapi.URI,
	langIDs []string, prefix string,
) iterator.Iterator[string] {
	ctx, cancel := context.WithCancel(context.Background())
	// Buffered so the producer can stay ahead of a slow consumer
	// without blocking on every send.
	out := make(chan string, 16)
	go debug.CapturePanicReport(func() {
		defer close(out)
		seen := make(map[string]struct{})
		for _, langID := range langIDs {
			q, ok := entrypointQueries[langID]
			if !ok {
				continue
			}
			it, err := parser.Search(q.query, q.captures, langID)
			if err != nil {
				continue
			}
			drainSearch(ctx, it, q, root, prefix, seen, out)
			if ctx.Err() != nil {
				return
			}
		}
	})
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case path, ok := <-out:
			return path, ok, nil
		}
	}, func() error {
		cancel()
		return nil
	})
}

func drainSearch(
	ctx context.Context, it iterator.Iterator[syntaxapi.Result],
	q entrypointQuery, root workspaceapi.URI, prefix string,
	seen map[string]struct{}, out chan<- string,
) {
	defer it.Close() //nolint:errcheck
	for {
		r, ok := it.Next(ctx)
		if !ok {
			return
		}
		if !q.match(r) {
			continue
		}
		path := workspaceapi.RelPath(root, r.File)
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		select {
		case <-ctx.Done():
			return
		case out <- path:
		}
	}
}
