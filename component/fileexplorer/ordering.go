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
