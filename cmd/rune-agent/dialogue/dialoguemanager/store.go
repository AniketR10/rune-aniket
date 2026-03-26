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

package dialoguemanager

import (
	"context"
	"fmt"
	"slices"
	"time"

	"unstable.build/go-tui/cmd/rune-agent/llm"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/retry"
)

var retryStrategy = retry.CombinedStrategy(
	retry.ExponentialStrategy(10*time.Millisecond, 500*time.Millisecond),
	retry.LimitStrategy(10),
)

// Store abstracts dialogue persistence to durable storage.
type Store interface {
	Health(context.Context) error
	Create(context.Context, Dialogue) error
	Get(context.Context, string) (Dialogue, error)
	Delete(context.Context, string) error
	AppendMessages(context.Context, Dialogue, []llm.Message, llm.DialogueUsage) error
	ArchiveAndReplace(context.Context, ArchiveAndReplaceParams) error
	List(context.Context) (iterator.Iterator[DialogueHeader], error)
}

// NewStore allocates storage for a new Store and
// initializes it with the given llm.
func NewStore(backend storageapi.Service) Store {
	return store{backend: backend}
}

type store struct {
	backend storageapi.Service
}

// Dialogue holds a dialogue's data as stored in durable storage.
type Dialogue struct {
	ID           string
	AgentID      string
	Model        string
	WorkspaceURI string
	SubAgent     bool
	Version      int
	MessageCount int
	Messages     []llm.Message
	Usage        llm.DialogueUsage
	UpdatedAt    time.Time
}

// DialogueHeader holds the metadata of a dialogue without the full message
// history. Use this for listing/filtering dialogues without paying the
// deserialization cost of Messages.
type DialogueHeader struct {
	ID           string
	AgentID      string
	Model        string
	WorkspaceURI string
	SubAgent     bool
	Version      int
	MessageCount int
	Usage        llm.DialogueUsage
	UpdatedAt    time.Time
}

// Workspace parses WorkspaceURI and returns the resulting URI.
// The second return value is false when WorkspaceURI is empty or
// cannot be parsed.
func (d Dialogue) Workspace() (workspaceapi.URI, bool) {
	if d.WorkspaceURI == "" {
		return workspaceapi.URI{}, false
	}
	u, err := workspaceapi.ParseURI(d.WorkspaceURI)
	if err != nil {
		return workspaceapi.URI{}, false
	}
	return u, true
}

// Workspace parses WorkspaceURI and returns the resulting URI.
// The second return value is false when WorkspaceURI is empty or
// cannot be parsed.
func (d DialogueHeader) Workspace() (workspaceapi.URI, bool) {
	if d.WorkspaceURI == "" {
		return workspaceapi.URI{}, false
	}
	u, err := workspaceapi.ParseURI(d.WorkspaceURI)
	if err != nil {
		return workspaceapi.URI{}, false
	}
	return u, true
}

// Header returns a DialogueHeader from this Dialogue.
func (d Dialogue) Header() DialogueHeader {
	return DialogueHeader{
		ID:           d.ID,
		AgentID:      d.AgentID,
		Model:        d.Model,
		WorkspaceURI: d.WorkspaceURI,
		SubAgent:     d.SubAgent,
		Version:      d.Version,
		MessageCount: d.MessageCount,
		Usage:        d.Usage,
		UpdatedAt:    d.UpdatedAt,
	}
}

// ArchiveAndReplaceParams holds the parameters for an ArchiveAndReplace
// operation. The caller provides the current Dialogue (which it already
// has in memory) and only specifies the new messages. Stable metadata
// (AgentID, Model, WorkspaceURI, Usage) is copied from the provided Dialogue.
type ArchiveAndReplaceParams struct {
	// Dialogue is the current dialogue being compacted/cleared.
	// Its full content is archived under ArchivedDialogueID.
	Dialogue Dialogue
	// ArchivedDialogueID is the ID under which the old messages are archived.
	ArchivedDialogueID string
	// Messages is the replacement message slice for the active dialogue.
	Messages []llm.Message
}

func (s store) Health(ctx context.Context) error {
	err := s.backend.Delete(ctx, "IDThatWillNeverExist")
	if err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	return nil
}

func (s store) Create(ctx context.Context, d Dialogue) error {
	if d.Version == 0 {
		d.Version = 1
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = time.Now()
	}
	d.MessageCount = len(d.Messages)
	err := s.backend.Create(ctx, d.ID, &d)
	if err != nil {
		return fmt.Errorf("document service create: %w", err)
	}
	return err
}

func (s store) set(ctx context.Context, d Dialogue) error {
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = time.Now()
	}
	d.Version = 1
	d.MessageCount = len(d.Messages)

	err := s.backend.Set(ctx, d.ID, &d)
	if err != nil {
		return fmt.Errorf("document service set: %w", err)
	}
	return nil
}

func (s store) ArchiveAndReplace(ctx context.Context, p ArchiveAndReplaceParams) error {
	archived := p.Dialogue
	archived.ID = p.ArchivedDialogueID
	if err := s.Create(ctx, archived); err != nil {
		return fmt.Errorf("archive-and-replace archive: %w", err)
	}

	replaced := p.Dialogue
	replaced.Messages = p.Messages
	replaced.MessageCount = len(p.Messages)
	replaced.Version = 1
	replaced.UpdatedAt = time.Now()
	if err := s.set(ctx, replaced); err != nil {
		return fmt.Errorf("archive-and-replace replace: %w", err)
	}
	return nil
}

func (s store) Get(
	ctx context.Context, ID string,
) (Dialogue, error) {
	var doc Dialogue
	err := s.backend.Get(ctx, ID, &doc)
	if err != nil {
		return Dialogue{}, fmt.Errorf("document service get: %w", err)
	}
	return doc, nil
}

func (s store) Delete(
	ctx context.Context, ID string,
) error {
	err := s.backend.Delete(ctx, ID)
	if err != nil {
		return fmt.Errorf("document service delete: %w", err)
	}
	return nil
}

func (s store) AppendMessages(
	ctx context.Context, d Dialogue, msgs []llm.Message, usage llm.DialogueUsage,
) error {
	err := storageapi.ConsistentUpdate(ctx, s.backend, d.ID, &d, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			d.Messages = append(d.Messages, msgs...)
			messageCount := len(d.Messages)

			accumulated := d.Usage
			accumulated.TokensSent += usage.TokensSent
			accumulated.TokensReceived += usage.TokensReceived
			accumulated.TokensReasoned += usage.TokensReasoned
			accumulated.TokensCached += usage.TokensCached
			accumulated.Completions += usage.Completions
			accumulated.ToolCalls += usage.ToolCalls
			accumulated.TotalDuration += usage.TotalDuration
			accumulated.InferenceDuration += usage.InferenceDuration
			accumulated.ToolCallDuration += usage.ToolCallDuration

			updatedAt := time.Now()
			return []storageapi.Update{
					{FieldPath: []string{"Messages"}, Value: d.Messages},
					{FieldPath: []string{"MessageCount"}, Value: messageCount},
					{FieldPath: []string{"Usage"}, Value: accumulated},
					{FieldPath: []string{"UpdatedAt"}, Value: updatedAt},
					{FieldPath: []string{"Version"}, Value: d.Version + 1},
				}, []storageapi.Precondition{
					{FieldPath: []string{"Version"}, Value: d.Version},
				}
		})
	if err != nil {
		return fmt.Errorf("consistent update : %w", err)
	}
	return nil
}

func (s store) List(ctx context.Context) (iterator.Iterator[DialogueHeader], error) {
	it, err := s.backend.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	all, err := iterator.ToSlice(ctx, iterator.FromDocumentIterator[DialogueHeader](it))
	if err != nil {
		return nil, fmt.Errorf("collect dialogues: %w", err)
	}
	// Sort by UpdatedAt descending (most recently updated first / LIFO).
	slices.SortFunc(all, func(a, b DialogueHeader) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return iterator.FromSlice(all), nil
}
