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
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// MultiQuery is one tree-sitter query in a SearchMulti batch. ID tags the
// query so results from a single shared workspace walk can be demultiplexed
// back to the query that produced them.
type MultiQuery struct {
	ID       int
	Query    string
	Captures []string
}

// MultiResult is a single SearchMulti capture tagged with the ID of the
// MultiQuery that produced it.
type MultiResult struct {
	QueryID int
	Result  syntaxapi.Result
}

// Searcher is the subset of the workspace parser that symbol resolution
// needs. SearchMulti runs several queries against a single per-file parse
// so resolution issues one workspace walk instead of one per query.
type Searcher interface {
	// Search runs a single tree-sitter query across the workspace,
	// optionally restricted to the given languages.
	Search(query string, captureNames []string, languages ...string) (
		iterator.Iterator[syntaxapi.Result], error,
	)
	// SearchMulti runs every query against one shared parse per file and
	// streams tagged results, optionally restricted to the given languages.
	SearchMulti(queries []MultiQuery, languages ...string) (
		iterator.Iterator[MultiResult], error,
	)
	// SearchNode runs the built-in node-capture query across the workspace,
	// optionally restricted to the given languages.
	SearchNode(nodeTypes syntaxapi.NodeCaptureName, languages ...string) (
		iterator.Iterator[syntaxapi.Result], error,
	)
	// QueryNode runs the built-in node-capture query against a single file.
	QueryNode(file workspaceapi.URI, nodeTypes syntaxapi.NodeCaptureName) (
		iterator.Iterator[syntaxapi.Result], error,
	)
}
