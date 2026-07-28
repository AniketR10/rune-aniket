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

package dream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
)

func TestEphemeralStore(t *testing.T) {
	ctx := context.Background()

	t.Run("Health returns nil", func(t *testing.T) {
		s := newEphemeralStore()
		assert.NoError(t, s.Health(ctx))
	})

	t.Run("Create and Get", func(t *testing.T) {
		s := newEphemeralStore()
		msgs := []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "hello"},
		}
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{
			ID: "d1", Messages: msgs,
		}))

		d, err := s.Get(ctx, "d1")
		require.NoError(t, err)
		assert.Equal(t, "d1", d.ID)
		assert.Len(t, d.Messages, 1)
		assert.Equal(t, "hello", d.Messages[0].Content)
		assert.Equal(t, int64(1), d.Version)
	})

	t.Run("Create returns ErrAlreadyExists", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"}))

		err := s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"})
		assert.ErrorIs(t, err, storageapi.ErrAlreadyExists)
	})

	t.Run("Get returns ErrNotFound", func(t *testing.T) {
		s := newEphemeralStore()
		_, err := s.Get(ctx, "nonexistent")
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("Delete removes dialogue", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"}))
		require.NoError(t, s.Delete(ctx, "d1"))

		_, err := s.Get(ctx, "d1")
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("AppendMessages appends and increments version", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{
			ID: "d1", Messages: []llmapi.Message{
				{Role: llmapi.RoleUser, Content: "first"},
			},
		}))

		d, err := s.Get(ctx, "d1")
		require.NoError(t, err)

		err = s.AppendMessages(ctx, d, []llmapi.Message{
			{Role: llmapi.RoleAssistant, Content: "second"},
		}, llmapi.DialogueUsage{})
		require.NoError(t, err)

		d, err = s.Get(ctx, "d1")
		require.NoError(t, err)
		assert.Len(t, d.Messages, 2)
		assert.Equal(t, "second", d.Messages[1].Content)
		assert.Equal(t, int64(2), d.Version)
	})

	t.Run("AppendMessages returns ErrNotFound for missing dialogue", func(t *testing.T) {
		s := newEphemeralStore()
		err := s.AppendMessages(ctx, dialoguemanager.Dialogue{ID: "nope"}, nil, llmapi.DialogueUsage{})
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("List returns all dialogues", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"}))
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d2"}))

		it, err := s.List(ctx)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		var ids []string
		for {
			d, ok := it.Next(ctx)
			if !ok {
				break
			}
			ids = append(ids, d.ID)
		}
		assert.NoError(t, it.Err())
		assert.Len(t, ids, 2)
		assert.Contains(t, ids, "d1")
		assert.Contains(t, ids, "d2")
	})
}
