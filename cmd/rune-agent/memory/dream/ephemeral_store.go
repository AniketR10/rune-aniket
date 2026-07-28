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
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
)

var _ dialoguemanager.Store = (*ephemeralStore)(nil)

// ephemeralStore is an in-memory dialoguemanager.Store for disposable
// conversations. The dream agent's own internal conversation does not
// need to be persisted.
type ephemeralStore struct {
	mu   sync.Mutex
	data map[string]dialoguemanager.Dialogue
}

func newEphemeralStore() *ephemeralStore {
	return &ephemeralStore{data: make(map[string]dialoguemanager.Dialogue)}
}

func (s *ephemeralStore) Health(_ context.Context) error { return nil }

func (s *ephemeralStore) Create(
	_ context.Context, d dialoguemanager.Dialogue,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[d.ID]; ok {
		return storageapi.ErrAlreadyExists
	}
	if d.Version == 0 {
		d.Version = 1
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = time.Now()
	}
	s.data[d.ID] = d
	return nil
}

func (s *ephemeralStore) Get(
	_ context.Context, id string,
) (dialoguemanager.Dialogue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.data[id]
	if !ok {
		return dialoguemanager.Dialogue{}, storageapi.ErrNotFound
	}
	return d, nil
}

func (s *ephemeralStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

func (s *ephemeralStore) AppendMessages(
	_ context.Context, d dialoguemanager.Dialogue,
	msgs []llmapi.Message, _ llmapi.DialogueUsage,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.data[d.ID]
	if !ok {
		return storageapi.ErrNotFound
	}
	existing.Messages = append(existing.Messages, msgs...)
	existing.Version++
	existing.UpdatedAt = time.Now()
	s.data[d.ID] = existing
	return nil
}

func (s *ephemeralStore) List(_ context.Context) (iterator.Iterator[dialoguemanager.DialogueHeader], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	headers := make([]dialoguemanager.DialogueHeader, 0, len(s.data))
	for _, d := range s.data {
		headers = append(headers, d.Header())
	}
	return iterator.FromSlice(headers), nil
}

func (s *ephemeralStore) ArchiveAndReplace(_ context.Context, p dialoguemanager.ArchiveAndReplaceParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	archived := p.Dialogue
	archived.ID = p.ArchivedDialogueID
	s.data[p.ArchivedDialogueID] = archived
	replaced := p.Dialogue
	replaced.Messages = p.Messages
	replaced.MessageCount = len(p.Messages)
	replaced.Version = 1
	replaced.UpdatedAt = time.Now()
	s.data[p.Dialogue.ID] = replaced
	return nil
}
