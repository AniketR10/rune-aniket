// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"fmt"
	"testing"
	"time"

	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// benchDialogueStore creates a dialoguemanager.Store backed by an in-memory
// storage service, populated with n dialogues. Each dialogue has numMessages
// messages and belongs to the given workspace.
func benchDialogueStore(b *testing.B, n, numMessages int, wsURI string) dialoguemanager.Store {
	b.Helper()
	backend := storagestub.NewInMemoryService()
	store := dialoguemanager.NewStore(backend)

	msgs := make([]llm.Message, numMessages)
	for i := range msgs {
		msgs[i] = llm.Message{
			Role:    llm.RoleUser,
			Content: fmt.Sprintf("message content %d — this simulates a realistic user prompt with moderate length", i),
		}
		if i%2 == 1 {
			msgs[i].Role = llm.RoleAssistant
			msgs[i].Content = fmt.Sprintf("assistant response %d — this is a longer response to simulate real usage with code blocks and explanations that take up more space in the serialized form", i)
		}
	}

	ctx := context.Background()
	for i := 0; i < n; i++ {
		d := dialoguemanager.Dialogue{
			ID:        fmt.Sprintf("dialogue-%04d", i),
			Model:     "test-model",
			WorkspaceURI: wsURI,
			Messages:  msgs,
			UpdatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
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
			ID:        fmt.Sprintf("dialogue-%04d", i),
			Model:     "test-model",
			WorkspaceURI: wsURI,
			UpdatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
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

// BenchmarkMakeCommandCompleter measures the end-to-end Tab-completion path:
// makeCommandCompleter → Complete → Store.List → filter → collect → prefix filter.
// This is what the user experiences when pressing Tab in the input box.
func BenchmarkMakeCommandCompleter(b *testing.B) {
	ws, _ := workspaceapi.ParseURI("file:///test/workspace")

	for _, numDialogues := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("dialogues=%d/store=inmemory", numDialogues), func(b *testing.B) {
			store := benchDialogueStore(b, numDialogues, 10, ws.String())
			h := &aiEditorHandler{
				dialogueStore: store,
				cwd:           ws,
			}
			completer := makeCommandCompleter(context.Background(), &benchCommandAdapter{handler: h})

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				head, comps, tail := completer("/agent ", 7)
				if len(comps) != numDialogues {
					b.Fatalf("expected %d completions, got %d (head=%q tail=%q)", numDialogues, len(comps), head, tail)
				}
			}
		})

		b.Run(fmt.Sprintf("dialogues=%d/store=mock", numDialogues), func(b *testing.B) {
			mockStore := benchMockListStore(numDialogues, ws.String())
			h := &aiEditorHandler{
				dialogueStore: mockStore,
				cwd:           ws,
			}
			completer := makeCommandCompleter(context.Background(), &benchCommandAdapter{handler: h})

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				head, comps, tail := completer("/agent ", 7)
				if len(comps) != numDialogues {
					b.Fatalf("expected %d completions, got %d (head=%q tail=%q)", numDialogues, len(comps), head, tail)
				}
			}
		})
	}
}

// BenchmarkMakeCommandCompleter_WithPrefix measures Tab completion with a
// prefix filter applied (simulating the user typing part of a dialogue name).
func BenchmarkMakeCommandCompleter_WithPrefix(b *testing.B) {
	ws, _ := workspaceapi.ParseURI("file:///test/workspace")

	for _, numDialogues := range []int{100, 500} {
		b.Run(fmt.Sprintf("dialogues=%d", numDialogues), func(b *testing.B) {
			store := benchDialogueStore(b, numDialogues, 10, ws.String())
			h := &aiEditorHandler{
				dialogueStore: store,
				cwd:           ws,
			}
			completer := makeCommandCompleter(context.Background(), &benchCommandAdapter{handler: h})

			// Only ~2% of dialogues will match "dialogue-00" prefix.
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, comps, _ := completer("/agent dialogue-00", 18)
				_ = comps
			}
		})
	}
}

// benchCommandAdapter wraps aiEditorHandler to satisfy dialoguetui.CommandHandler
// for benchmarking the complete flow.
type benchCommandAdapter struct {
	handler *aiEditorHandler
}

func (a *benchCommandAdapter) HandleCommand(context.Context, string, []string) (dialoguetui.CommandResult, error) {
	return dialoguetui.CommandResult{}, nil
}

func (a *benchCommandAdapter) Complete(ctx context.Context, name string, args []string) (iterator.Iterator[string], error) {
	return a.handler.Complete(ctx, name, args)
}
