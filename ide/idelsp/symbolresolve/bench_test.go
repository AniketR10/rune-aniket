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

package symbolresolve_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/ide/idelsp/symbolresolve"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/workspace"
)

// setupBenchWorkspace writes a definition package plus n consumer files that
// all import and reference mylib.MyType, so resolving the symbol exercises the
// reference, package-clause and import (dedup) queries together.
func setupBenchWorkspace(b *testing.B, n int) syntax.Parser {
	b.Helper()

	root := b.TempDir()
	require.NoError(b, os.MkdirAll(filepath.Join(root, "mylib"), 0o755))
	require.NoError(b, os.WriteFile(
		filepath.Join(root, "mylib", "mylib.go"),
		[]byte("package mylib\n\ntype MyType struct{}\n"),
		0o644,
	))
	for i := range n {
		src := fmt.Sprintf(
			"package consumer%d\n\nimport \"example.com/bench/mylib\"\n\n"+
				"func use%d(v mylib.MyType) mylib.MyType { return v }\n",
			i, i,
		)
		require.NoError(b, os.WriteFile(
			filepath.Join(root, fmt.Sprintf("use%d.go", i)), []byte(src), 0o644,
		))
	}

	rootURI := "file://" + root
	uri, err := workspaceapi.ParseURI(rootURI)
	require.NoError(b, err)
	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
	require.NoError(b, err)
	b.Cleanup(func() { _ = scheme.Close() })

	return syntax.NewParser(scheme, treeSitterPkgManager(b), uri)
}

func BenchmarkResolve(b *testing.B) {
	for _, n := range []int{8, 32, 128} {
		b.Run(fmt.Sprintf("files=%d", n), func(b *testing.B) {
			parser := setupBenchWorkspace(b, n)
			b.ResetTimer()
			for range b.N {
				matches, err := symbolresolve.Resolve(
					context.Background(), parser, specIter(symbolresolve.Go),
					"mylib.MyType", nil,
				)
				require.NoError(b, err)
				require.NotEmpty(b, matches)
			}
		})
	}
}

// BenchmarkResolveSerialBaseline models the prior implementation's cost: the
// reference, package-clause and import path/alias queries each issued as their
// own full-workspace Search pass. It exists to quantify the single-pass win
// over the previous five serial walks.
func BenchmarkResolveSerialBaseline(b *testing.B) {
	for _, n := range []int{8, 32, 128} {
		b.Run(fmt.Sprintf("files=%d", n), func(b *testing.B) {
			parser := setupBenchWorkspace(b, n)
			spec := symbolresolve.Go
			passes := [][2]any{
				{spec.RefQueries[0].Query, spec.RefQueries[0].Captures},
				{spec.RefQueries[1].Query, spec.RefQueries[1].Captures},
				{spec.PackageClauseQuery, spec.PackageClauseCaptures},
				{spec.ImportPathQuery, spec.ImportPathCaptures},
				{spec.ImportAliasQuery, spec.ImportAliasCaptures},
			}
			b.ResetTimer()
			for range b.N {
				for _, p := range passes {
					it, err := parser.Search(p[0].(string), p[1].([]string), spec.LangID)
					require.NoError(b, err)
					_, err = iterator.ToSlice(context.Background(), it)
					require.NoError(b, err)
				}
			}
		})
	}
}
