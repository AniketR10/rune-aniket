// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package dialoguemanager

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestGenerateUniqueID(t *testing.T) {
	ctx := context.Background()

	t.Run("generates ID with prefix", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())
		id := GenerateUniqueID(ctx, store, "sub-agent-plan-")
		assert.True(t, strings.HasPrefix(id, "sub-agent-plan-"))
	})

	t.Run("generates ID without prefix", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())
		id := GenerateUniqueID(ctx, store, "")
		assert.NotEmpty(t, id)
		// Petname format: word-word
		assert.Contains(t, id, "-")
	})

	t.Run("retries on collision", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())

		// Generate first ID, then pre-create a dialogue with
		// the same ID to force a collision on next call.
		id1 := GenerateUniqueID(ctx, store, "test-")
		require.NoError(t, store.Create(ctx, Dialogue{ID: id1}))

		// Second call must produce a different ID.
		id2 := GenerateUniqueID(ctx, store, "test-")
		assert.NotEqual(t, id1, id2)
		assert.True(t, strings.HasPrefix(id2, "test-"))
	})

	t.Run("unique across multiple calls", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())
		seen := make(map[string]bool)
		for range 20 {
			id := GenerateUniqueID(ctx, store, "")
			assert.False(t, seen[id], "duplicate ID: %s", id)
			seen[id] = true
			// Persist each ID so the store check can find it.
			require.NoError(t, store.Create(ctx, Dialogue{ID: id}))
		}
	})
}
