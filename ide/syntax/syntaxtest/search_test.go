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
package syntaxtest

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/workspace"
)

//go:embed go/locals.scm
var locals []byte

func TestSearch(t *testing.T) {
	t.Run("returns functions", func(t *testing.T) {
		searcher, uri1, uri2, uri3 := setupSearcherForTests(t,
			"go/tree-sitter.so",
			"go/locals.scm",
		)
		it, err := searcher.Search(string(locals), []string{"local.definition.function"})
		require.NoError(t, err)
		funcs, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.ElementsMatch(t, []syntaxapi.Result{
			{File: uri1, CaptureName: "local.definition.function",
				From: term.Coordinates{X: 5, Y: 3},
				To:   term.Coordinates{X: 13, Y: 3},
				Text: "function",
			},
			{File: uri2, CaptureName: "local.definition.function",
				From: term.Coordinates{X: 5, Y: 3},
				To:   term.Coordinates{X: 13, Y: 3},
				Text: "function",
			},
			{File: uri3, CaptureName: "local.definition.function",
				From: term.Coordinates{X: 5, Y: 3},
				To:   term.Coordinates{X: 13, Y: 3},
				Text: "function",
			},
		}, funcs)
	})

	t.Run("returns all captures if not capture names are passed", func(t *testing.T) {
		searcher, _, _, _ := setupSearcherForTests(t,
			"go/tree-sitter.so",
			"go/locals.scm",
		)
		it, err := searcher.Search(string(locals), []string{})
		require.NoError(t, err)
		funcs, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Len(t, funcs, 45)
	})

	t.Run("returns nothing if query capture name matches nothing", func(t *testing.T) {
		searcher, _, _, _ := setupSearcherForTests(t,
			"go/tree-sitter.so",
			"go/locals.scm",
		)
		it, err := searcher.Search(string(locals), []string{"local.definition.NOTHING"})
		require.NoError(t, err)
		funcs, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		assert.Len(t, funcs, 0)
	})

	t.Run("returns error if query is not valid SCM", func(t *testing.T) {
		searcher, _, _, _ := setupSearcherForTests(t,
			"go/tree-sitter.so",
			"go/locals.scm",
		)
		invalidQuery := `((identifier @id)
 (#eq? @id "x"`
		it, err := searcher.Search(invalidQuery, []string{"local.definition.function"})
		assert.NoError(t, err)
		_, err = iterator.ToSlice(context.Background(), it)
		require.Error(t, err)
	})
}

func createFile(t *testing.T, scheme schemeapi.Scheme, name string, content string) workspaceapi.URI {
	f, err := scheme.Create(name)
	require.NoError(t, err)

	_, err = f.Write([]byte(content))
	require.NoError(t, err)

	uri, err := scheme.URI(f.Name())
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return uri
}

func setupSearcherForTests(t *testing.T, filesAvail ...string) (
	searcher syntaxapi.Searcher, uri1, uri2, uri3 workspaceapi.URI,
) {
	logrus.SetLevel(logrus.TraceLevel)
	uri, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	wd, err := os.Getwd()
	require.NoError(t, err)
	var fullPathFiles []string
	for _, file := range filesAvail {
		if !filepath.IsAbs(file) {
			fullPathFiles = append(fullPathFiles, filepath.Join(wd, file))
		} else {
			fullPathFiles = append(fullPathFiles, file)
		}
	}
	pkgs := &mockPkgManager{
		fullPathFiles: fullPathFiles,
	}

	scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)

	uri1 = createFile(t, scheme, "a.go", searchFileContent)
	uri2 = createFile(t, scheme, "b.go", searchFileContent)
	uri3 = createFile(t, scheme, "c.go", searchFileContent)
	searcher = syntax.NewSearcher(scheme, pkgs, uri)
	return
}

const searchFileContent = `
package pkg

func function() string {
}

var a int
	
type myType struct {
	a string
}
`
