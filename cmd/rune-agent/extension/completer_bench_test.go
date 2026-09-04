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
	"fmt"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguemanager"
)

// benchDialogueStore creates a dialoguemanager.Store backed by an in-memory
// storage service, populated with n dialogues. Each dialogue has numMessages
// messages and belongs to the given workspace.
func benchDialogueStore(b *testing.B, n, numMessages int, wsURI string) dialoguemanager.Store {
	b.Helper()
	backend := storagestub.NewInMemoryService()
	store := dialoguemanager.NewStore(backend, b.TempDir())

	msgs := make([]llmapi.Message, numMessages)
	for i := range msgs {
		msgs[i] = llmapi.Message{
			Role:    llmapi.RoleUser,
			Content: fmt.Sprintf("message content %d — this simulates a realistic user prompt with moderate length", i),
		}
		if i%2 == 1 {
			msgs[i].Role = llmapi.RoleAssistant
			msgs[i].Content = fmt.Sprintf("assistant response %d — this is a longer response to simulate real usage with code blocks and explanations that take up more space in the serialized form", i)
		}
	}

	ctx := context.Background()
	for i := 0; i < n; i++ {
		d := dialoguemanager.Dialogue{
			ID:           fmt.Sprintf("dialogue-%04d", i),
			Model:        "test-model",
			WorkspaceURI: wsURI,
			Messages:     msgs,
			UpdatedAt:    time.Now().Add(-time.Duration(i) * time.Minute),
		}
		if err := store.Create(ctx, d); err != nil {
			b.Fatal(err)
		}
	}
	return store
}

// benchMockListStore returns a mock store that returns pre-built dialogues
// from a slice (no serialization/deserialization). This isolates the sort +
// filter + map cost from the storage I/O cost.
func benchMockListStore(n int, wsURI string) *fakeListStore {
	dialogues := make([]dialoguemanager.DialogueHeader, n)
	for i := range dialogues {
		dialogues[i] = dialoguemanager.DialogueHeader{
			ID:           fmt.Sprintf("dialogue-%04d", i),
			Model:        "test-model",
			WorkspaceURI: wsURI,
			UpdatedAt:    time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}
	return &fakeListStore{dialogues: dialogues}
}

// fakeListStore is defined in gitidentity_test.go. Re-using it here.

// BenchmarkStoreList measures the cost of dialoguemanager.Store.List which
// reads all dialogues from the storage backend, deserializes them, and sorts
// by UpdatedAt descending.
//
// This is the single most expensive operation in the completer hot path.
func BenchmarkStoreList(b *testing.B) {
	ws, _ := workspaceapi.ParseURI("file:///test/workspace")

	for _, numDialogues := range []int{10, 50, 100, 500} {
		for _, numMessages := range []int{0, 10, 50} {
			name := fmt.Sprintf("dialogues=%d/messages=%d", numDialogues, numMessages)
			b.Run(name, func(b *testing.B) {
				store := benchDialogueStore(b, numDialogues, numMessages, ws.String())
				ctx := context.Background()

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					it, err := store.List(ctx)
					if err != nil {
						b.Fatal(err)
					}
					all, err := iterator.ToSlice(ctx, it)
					if err != nil {
						b.Fatal(err)
					}
					if len(all) != numDialogues {
						b.Fatalf("expected %d dialogues, got %d", numDialogues, len(all))
					}
				}
			})
		}
	}
}

// BenchmarkCompleteWithDialoguesIterator measures the full
// completeWithDialoguesIterator path: Store.List → filter by workspace →
// map to string IDs → collect to slice.
func BenchmarkCompleteWithDialoguesIterator(b *testing.B) {
	ws, _ := workspaceapi.ParseURI("file:///test/workspace")

	for _, numDialogues := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("dialogues=%d/store=inmemory", numDialogues), func(b *testing.B) {
			store := benchDialogueStore(b, numDialogues, 10, ws.String())
			h := &aiEditorHandler{
				dialogueStore: store,
				cwd:           ws,
			}
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				it, err := h.completeWithDialoguesIterator(ctx, false)
				if err != nil {
					b.Fatal(err)
				}
				all, err := iterator.ToSlice(ctx, it)
				if err != nil {
					b.Fatal(err)
				}
				if len(all) != numDialogues {
					b.Fatalf("expected %d, got %d", numDialogues, len(all))
				}
			}
		})

		// Same but with a mock store to isolate filtering/mapping cost from storage I/O.
		b.Run(fmt.Sprintf("dialogues=%d/store=mock", numDialogues), func(b *testing.B) {
			mockStore := benchMockListStore(numDialogues, ws.String())
			h := &aiEditorHandler{
				dialogueStore: mockStore,
				cwd:           ws,
			}
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				it, err := h.completeWithDialoguesIterator(ctx, false)
				if err != nil {
					b.Fatal(err)
				}
				all, err := iterator.ToSlice(ctx, it)
				if err != nil {
					b.Fatal(err)
				}
				if len(all) != numDialogues {
					b.Fatalf("expected %d, got %d", numDialogues, len(all))
				}
			}
		})
	}
}

// (Tab-completion benchmarks removed along with the inputbox compose
// backend; the editor-backed compose input no longer uses a
// WordCompleter.)
