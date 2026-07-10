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

package syntax

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// closedTree simulates the window where a lazy iterator was handed out
// before the Tree was ready and Tree.Close freed the native
// tree-sitter objects before the iterator's first Next. The zero-value
// natives stand in for freed objects: any native call through them is
// a use-after-free in production and crashes here.
func closedTree(ready chan struct{}) *Tree {
	return &Tree{
		ready:        true,
		closed:       true,
		waitingReady: ready,
		tree:         &tree_sitter.Tree{},
		folds:        &tree_sitter.Query{},
	}
}

func TestNodesIteratorAfterTreeClose(t *testing.T) {
	ready := make(chan struct{})
	it := newNodesIterator(closedTree(ready), ready, "folds.scm")
	close(ready)

	_, ok := it.Next(context.Background())
	require.False(t, ok)
	assert.ErrorContains(t, it.Err(), "closed")
}

func TestFoldsIteratorAfterTreeClose(t *testing.T) {
	tests := []struct {
		name string
		make func(tree *Tree, ready chan struct{}) *foldsIterator
	}{
		{"folds", func(tree *Tree, ready chan struct{}) *foldsIterator {
			return newFoldsIterator(false, tree, ready)
		}},
		{"initial folds", func(tree *Tree, ready chan struct{}) *foldsIterator {
			return newFoldsIterator(true, tree, ready)
		}},
		{"folds from", func(tree *Tree, ready chan struct{}) *foldsIterator {
			return newFoldsFromIterator(term.Coordinates{Y: 1}, tree, ready)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready := make(chan struct{})
			it := tt.make(closedTree(ready), ready)
			close(ready)

			_, ok := it.Next(context.Background())
			assert.False(t, ok)
			assert.NoError(t, it.Err())
		})
	}
}
