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

package extension

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/handler/finder"
	"unstable.build/rune/internal/workspace/walkdir"
)

var cmdSearchSyntax = textapi.CommandManual{
	Name: "searchast",
	Summary: "Fuzzy search AST nodes by running the given query against all the files in " +
		"the workspace. The first argument is the name or path of the query file to run. " +
		"The second argument is the match capture name(s), and the last argument " +
		"is the name of the node in the file (i.e. function name, variable name, etc.). " +
		"The second argument can be ORed by adding a `|` character between match names." +
		"For example to match against functions and methods you can pass: " +
		"locals.scm local.definition.method|local.definition.function. " +
		"The query file should be a relative or absolute path and if not found, " +
		"it will be searched in the file's language package installation " +
		"folder.",
	Synopsis: "<query> <capture>...",
}

func readSymbolsFunction(dataDir string, queryFile string, captureNames []string) func(
	workspaceapi.FileSystem, context.Context) (iterator.Iterator[string], error,
) {
	return func(cwd workspaceapi.FileSystem, ctx context.Context) (
		iterator.Iterator[string], error,
	) {
		var q string
		switch queryFile {
		case "folds.scm", "indents.scm",
			"highlights.scm", "locals.scm":
			// empty query instructs readSymbols to find .scm file in lang lib
		default:
			data, err := os.ReadFile(queryFile)
			if err != nil {
				return nil, fmt.Errorf("read query file: %w", err)
			}
			q = string(data)
		}
		it, err := walkdir.ListFiles(ctx, cwd, ".")
		if err != nil {
			return nil, err
		}
		uri, err := cwd.URI(".")
		if err != nil {
			return nil, err
		}
		sit, err := readSymbols(ctx, cwd, dataDir, uri, it, queryFile, q)
		if err != nil {
			return nil, err
		}
		if len(captureNames) != 0 {
			sit = iterator.Filter(sit, func(m match) bool {
				return slices.Contains(captureNames, m.CaptureName)
			})
		}
		return iterator.Map(sit, func(m match) string { return m.LineString }), nil
	}
}

func syntaxResource(exec workspaceapi.FileSystem, data string) (
	uri workspaceapi.URI, coords term.Coordinates, ok bool,
) {
	chunks := strings.Split(data, ":")
	if len(chunks) < 2 {
		return workspaceapi.URI{}, term.Coordinates{}, false
	}
	y, _ := strconv.Atoi(chunks[1])
	name := chunks[0]
	uri, err := exec.URI(name)
	return uri, term.Coordinates{Y: y - 1}, err == nil
}

func newSyntaxHandler(
	ctx context.Context, cmd textapi.Command,
	clients finder.Clients, invokeWindow browserapi.Window, c config.Config, dataDir string,
) (finder.RedispatchHandler, error) {
	if len(cmd.Args) < 2 {
		return nil, errors.New("expected at least three arguments")
	}
	queryFile, captureNameString := cmd.Args[0], cmd.Args[1]
	captureNames := strings.Split(captureNameString, "|")
	noHistoryKey := term.KeyComb{}

	return finder.New(ctx, clients, invokeWindow,
		c, noHistoryKey, "unused", "",
		readSymbolsFunction(dataDir, queryFile, captureNames), syntaxResource)
}
