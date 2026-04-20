// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package fileexplorer

import (
	"slices"
	"strings"
)

// OrderOperations sorts operations to ensure correct execution order:
// 1. Deletes - deepest paths first (children before parents)
// 2. Renames/Moves - ordered to avoid conflicts
// 3. Creates/Mkdirs - shallowest paths first (parents before children)
// 4. Copies - after creates (source must exist)
func OrderOperations(ops []Operation) []Operation {
	if len(ops) == 0 {
		return ops
	}

	var deletes, renames, creates, copies []Operation
	for _, op := range ops {
		switch op.Type {
		case OpDelete:
			deletes = append(deletes, op)
		case OpRename, OpMove:
			renames = append(renames, op)
		case OpCreate, OpMkdir:
			creates = append(creates, op)
		case OpCopy:
			copies = append(copies, op)
		}
	}

	deletes = orderDeletes(deletes)
	renames = orderRenames(renames)
	creates = orderCreates(creates)
	copies = orderCreates(copies)

	result := make([]Operation, 0, len(ops))
	result = append(result, deletes...)
	result = append(result, renames...)
	result = append(result, creates...)
	result = append(result, copies...)
	return result
}

func orderDeletes(ops []Operation) []Operation {
	slices.SortFunc(ops, func(a, b Operation) int {
		depthA := pathDepth(a.URI.Path())
		depthB := pathDepth(b.URI.Path())
		return depthB - depthA
	})
	return ops
}

func orderRenames(ops []Operation) []Operation {
	if len(ops) <= 1 {
		return ops
	}

	graph := make(map[int][]int)
	for i, op := range ops {
		for j, other := range ops {
			if i == j {
				continue
			}
			if op.URI.Path() == other.NewURI.Path() {
				graph[i] = append(graph[i], j)
			}
		}
	}

	result := make([]Operation, 0, len(ops))
	visited := make(map[int]bool)
	inStack := make(map[int]bool)

	var visit func(int) bool
	visit = func(i int) bool {
		if inStack[i] {
			return false
		}
		if visited[i] {
			return true
		}
		inStack[i] = true
		for _, dep := range graph[i] {
			if !visit(dep) {
				return false
			}
		}
		inStack[i] = false
		visited[i] = true
		result = append(result, ops[i])
		return true
	}

	for i := range ops {
		visit(i)
	}

	return result
}

func orderCreates(ops []Operation) []Operation {
	slices.SortFunc(ops, func(a, b Operation) int {
		depthA := pathDepth(a.NewURI.Path())
		depthB := pathDepth(b.NewURI.Path())
		return depthA - depthB
	})
	return ops
}

func pathDepth(path string) int {
	path = strings.Trim(path, "/")
	if path == "" {
		return 0
	}
	return strings.Count(path, "/") + 1
}
