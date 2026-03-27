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

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/retry"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

const storeIndexRecordID = "dialogue-index"

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

type dialogueIndex struct {
	Headers map[string]DialogueHeader
	Version int
	Bootstrapped bool
}

func newDialogueIndex() dialogueIndex {
	return dialogueIndex{Headers: make(map[string]DialogueHeader), Version: 1}
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

func (s store) getIndex(ctx context.Context) (dialogueIndex, bool, error) {
	var idx dialogueIndex
	err := s.backend.Get(ctx, storeIndexRecordID, &idx)
	if err == nil {
		if idx.Headers == nil {
			idx.Headers = make(map[string]DialogueHeader)
		}
		if idx.Version == 0 {
			idx.Version = 1
		}
		return idx, true, nil
	}
	if err != storageapi.ErrNotFound {
		return dialogueIndex{}, false, fmt.Errorf("document service get index: %w", err)
	}
	return newDialogueIndex(), false, nil
}

func (s store) loadLegacyHeaders(ctx context.Context) (map[string]DialogueHeader, error) {
	it, err := s.backend.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("document service list legacy dialogues: %w", err)
	}
	all, err := iterator.ToSlice(ctx, iterator.FromDocumentIterator[Dialogue](it))
	if err != nil {
		return nil, fmt.Errorf("collect legacy dialogues: %w", err)
	}
	headers := make(map[string]DialogueHeader)
	for _, d := range all {
		if !isLegacyDialogue(d) || d.ID == storeIndexRecordID {
			continue
		}
		headers[d.ID] = d.Header()
	}
	return headers, nil
}

func (s store) ensureIndex(ctx context.Context) (dialogueIndex, error) {
	return s.ensureIndexMode(ctx, true)
}

func (s store) ensureIndexWithoutBootstrap(ctx context.Context) (dialogueIndex, error) {
	return s.ensureIndexMode(ctx, false)
}

func (s store) ensureIndexMode(ctx context.Context, bootstrap bool) (dialogueIndex, error) {
	idx, exists, err := s.getIndex(ctx)
	if err != nil {
		return dialogueIndex{}, err
	}
	if exists && idx.Bootstrapped {
		return idx, nil
	}
	if !bootstrap {
		if !exists {
			idx = newDialogueIndex()
			if err := s.backend.Create(ctx, storeIndexRecordID, &idx); err == nil {
				return idx, nil
			} else if err != storageapi.ErrAlreadyExists {
				return dialogueIndex{}, fmt.Errorf("document service create index: %w", err)
			}
			idx, _, err = s.getIndex(ctx)
			if err != nil {
				return dialogueIndex{}, err
			}
		}
		return idx, nil
	}

	legacyHeaders, err := s.loadLegacyHeaders(ctx)
	if err != nil {
		return dialogueIndex{}, err
	}

	if !exists {
		idx = newDialogueIndex()
		idx.Bootstrapped = true
		for id, header := range legacyHeaders {
			idx.Headers[id] = header
		}
		if err := s.backend.Create(ctx, storeIndexRecordID, &idx); err == nil {
			return idx, nil
		} else if err != storageapi.ErrAlreadyExists {
			return dialogueIndex{}, fmt.Errorf("document service create index: %w", err)
		}
	}

	err = storageapi.ConsistentUpdate(ctx, s.backend, storeIndexRecordID, &idx, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if idx.Headers == nil {
				idx.Headers = make(map[string]DialogueHeader)
			}
			for id, header := range legacyHeaders {
				if _, ok := idx.Headers[id]; !ok {
					idx.Headers[id] = header
				}
			}
			return []storageapi.Update{
				{FieldPath: []string{"Headers"}, Value: idx.Headers},
				{FieldPath: []string{"Bootstrapped"}, Value: true},
				{FieldPath: []string{"Version"}, Value: idx.Version + 1},
			}, []storageapi.Precondition{
				{FieldPath: []string{"Version"}, Value: idx.Version},
			}
		})
	if err != nil {
		return dialogueIndex{}, fmt.Errorf("document service bootstrap index: %w", err)
	}
	idx, _, err = s.getIndex(ctx)
	if err != nil {
		return dialogueIndex{}, err
	}
	return idx, nil
}

func (s store) updateIndex(ctx context.Context, fn func(dialogueIndex) dialogueIndex) error {
	idx, err := s.ensureIndexWithoutBootstrap(ctx)
	if err != nil {
		return err
	}
	return storageapi.ConsistentUpdate(ctx, s.backend, storeIndexRecordID, &idx, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			idx = fn(idx)
			return []storageapi.Update{
				{FieldPath: []string{"Headers"}, Value: idx.Headers},
				{FieldPath: []string{"Bootstrapped"}, Value: idx.Bootstrapped},
				{FieldPath: []string{"Version"}, Value: idx.Version + 1},
			}, []storageapi.Precondition{
				{FieldPath: []string{"Version"}, Value: idx.Version},
			}
		})
}

func isLegacyDialogue(d Dialogue) bool {
	return d.ID != "" && !d.UpdatedAt.IsZero()
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
	err = s.updateIndex(ctx, func(idx dialogueIndex) dialogueIndex {
		idx.Headers[d.ID] = d.Header()
		return idx
	})
	if err != nil {
		_ = s.backend.Delete(ctx, d.ID)
		return err
	}
	return nil
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
	err = s.updateIndex(ctx, func(idx dialogueIndex) dialogueIndex {
		idx.Headers[d.ID] = d.Header()
		return idx
	})
	if err != nil {
		return err
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
	err = s.updateIndex(ctx, func(idx dialogueIndex) dialogueIndex {
		delete(idx.Headers, ID)
		return idx
	})
	if err != nil {
		return err
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
	err = s.updateIndex(ctx, func(idx dialogueIndex) dialogueIndex {
		idx.Headers[d.ID] = d.Header()
		return idx
	})
	if err != nil {
		return err
	}
	return nil
}

func (s store) List(ctx context.Context) (iterator.Iterator[DialogueHeader], error) {
	idx, err := s.ensureIndex(ctx)
	if err != nil {
		return nil, err
	}
	all := make([]DialogueHeader, 0, len(idx.Headers))
	for _, header := range idx.Headers {
		all = append(all, header)
	}
	// Sort by UpdatedAt descending (most recently updated first / LIFO).
	slices.SortFunc(all, func(a, b DialogueHeader) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})
	return iterator.FromSlice(all), nil
}
