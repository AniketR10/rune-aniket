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

package llm

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// auditDialogueIDKey is the context key for passing the dialogue ID
// through to the audit service.
type auditDialogueIDKey struct{}

// WithAuditDialogueID returns a context carrying the given dialogue ID
// for audit logging.
func WithAuditDialogueID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, auditDialogueIDKey{}, id)
}

func auditDialogueIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(auditDialogueIDKey{}).(string)
	return v
}

// AuditEntry records a single LLM completion request and its outcome.
type AuditEntry struct {
	StartedAt       time.Time     `json:"StartedAt"`
	Duration        time.Duration `json:"Duration"`
	Model           string        `json:"Model,omitempty"`
	Provider        string        `json:"Provider,omitempty"`
	Messages        []Message     `json:"Messages"`
	Tools           int           `json:"Tools"`
	ToolNames       []string      `json:"ToolNames,omitempty"`
	ReasoningEffort string        `json:"ReasoningEffort,omitempty"`
	MaxOutputTokens int           `json:"MaxOutputTokens,omitempty"`
	EstimatedTokens int           `json:"EstimatedTokens"`
	ContextWindow   int           `json:"ContextWindow"`
	Response        *Message      `json:"Response,omitempty"`
	Usage           Usage         `json:"Usage"`
	FinishReason    FinishReason  `json:"FinishReason,omitempty"`
	Err             string        `json:"Err,omitempty"`
}

// auditDocument is the persisted document for a dialogue's audit log.
type auditDocument struct {
	Entries []AuditEntry `json:"Entries"`
}

const auditKeyPrefix = "audit:"

// AuditStore persists audit entries per dialogue using storageapi.
type AuditStore struct {
	backend storageapi.Service
}

// NewAuditStore returns a store backed by the given storage service.
func NewAuditStore(backend storageapi.Service) *AuditStore {
	return &AuditStore{backend: backend}
}

// Append adds an entry to the audit log for the given dialogue.
// It is safe to call on a nil receiver or with an empty dialogue ID.
func (s *AuditStore) Append(ctx context.Context, dialogueID string, entry AuditEntry) {
	if s == nil || dialogueID == "" {
		return
	}
	key := auditKeyPrefix + dialogueID
	var doc auditDocument
	if err := s.backend.Get(ctx, key, &doc); err != nil && !errors.Is(err, storageapi.ErrNotFound) {
		slog.Warn("llm.audit: load audit log", "dialogue", dialogueID, "error", err)
		return
	}
	doc.Entries = append(doc.Entries, entry)
	if err := s.backend.Set(ctx, key, &doc); err != nil {
		slog.Warn("llm.audit: save audit log", "dialogue", dialogueID, "error", err)
	}
}

// Get returns all audit entries for the given dialogue.
// Returns nil, nil when there are no entries or the receiver is nil.
func (s *AuditStore) Get(ctx context.Context, dialogueID string) ([]AuditEntry, error) {
	if s == nil {
		return nil, nil
	}
	key := auditKeyPrefix + dialogueID
	var doc auditDocument
	if err := s.backend.Get(ctx, key, &doc); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return doc.Entries, nil
}

// auditService wraps a Service, logging and recording token usage
// for every completion request.
type auditService struct {
	inner    Service
	store    *AuditStore
	model    string
	provider string
}

// NewAuditService wraps inner in an audit decorator that logs every
// completion request via slog and records entries in store.
// The model and provider strings are recorded in each audit entry.
func NewAuditService(inner Service, store *AuditStore, model, provider string) Service {
	return &auditService{inner: inner, store: store, model: model, provider: provider}
}

// CountTokens delegates to the inner service.
func (s *auditService) CountTokens(msgs []Message) (int, error) {
	return s.inner.CountTokens(msgs)
}

// ContextWindow delegates to the inner service.
func (s *auditService) ContextWindow() int {
	return s.inner.ContextWindow()
}

// CreateCompletion delegates to the inner service and wraps the returned
// iterator to intercept completion events for audit logging.
func (s *auditService) CreateCompletion(
	ctx context.Context, req Request,
) (iterator.Iterator[Event], error) {
	dialogueID := auditDialogueIDFrom(ctx)
	estimated, _ := s.inner.CountTokens(req.Messages)
	ctxWindow := s.inner.ContextWindow()

	slog.Info("llm.audit: completion started",
		"dialogue", dialogueID,
		"estimated_tokens_sent", estimated,
		"messages", len(req.Messages),
		"tools", len(req.Tools),
		"context_window", ctxWindow,
	)

	start := time.Now()

	toolNames := extractToolNames(req.Tools)

	it, err := s.inner.CreateCompletion(ctx, req)
	if err != nil {
		elapsed := time.Since(start)
		slog.Warn("llm.audit: completion error", "error", err, "duration", elapsed)
		s.store.Append(context.WithoutCancel(ctx), dialogueID, AuditEntry{
			StartedAt:       start,
			Duration:        elapsed,
			Model:           s.model,
			Provider:        s.provider,
			Messages:        copyMessages(req.Messages),
			Tools:           len(req.Tools),
			ToolNames:       toolNames,
			ReasoningEffort: string(req.ReasoningEffort),
			MaxOutputTokens: req.MaxOutputTokens,
			EstimatedTokens: estimated,
			ContextWindow:   ctxWindow,
			Err:             err.Error(),
		})
		return nil, err
	}

	entry := &AuditEntry{
		StartedAt:       start,
		Model:           s.model,
		Provider:        s.provider,
		Messages:        copyMessages(req.Messages),
		Tools:           len(req.Tools),
		ToolNames:       toolNames,
		ReasoningEffort: string(req.ReasoningEffort),
		MaxOutputTokens: req.MaxOutputTokens,
		EstimatedTokens: estimated,
		ContextWindow:   ctxWindow,
	}

	// Use a context without cancellation for storage writes so
	// the audit entry is persisted even if the request is cancelled.
	storeCtx := context.WithoutCancel(ctx)

	var recorded bool
	return iterator.FromFunc(
		func(ctx context.Context) (Event, bool, error) {
			ev, ok := it.Next(ctx)
			if !ok {
				if !recorded {
					recorded = true
					entry.Duration = time.Since(start)
					if itErr := it.Err(); itErr != nil {
						entry.Err = itErr.Error()
						slog.Warn("llm.audit: completion error",
							"error", itErr, "duration", entry.Duration)
					}
					s.store.Append(storeCtx, dialogueID, *entry)
				}
				return Event{}, false, it.Err()
			}
			switch ev.Type {
			case EventStreamDone:
				if ev.DoneData != nil && !recorded {
					recorded = true
					entry.Response = &ev.DoneData.Message
					entry.Usage = ev.DoneData.Usage
					entry.FinishReason = ev.DoneData.FinishReason
					entry.Duration = time.Since(start)
					slog.Info("llm.audit: completion done",
						"dialogue", dialogueID,
						"tokens_sent", ev.DoneData.Usage.TokensSent,
						"tokens_received", ev.DoneData.Usage.TokensReceived,
						"tokens_reasoned", ev.DoneData.Usage.TokensReasoned,
						"tokens_cached", ev.DoneData.Usage.TokensCached,
						"tokens_cache_created", ev.DoneData.Usage.TokensCacheCreated,
						"finish_reason", ev.DoneData.FinishReason,
						"duration", entry.Duration,
					)
					s.store.Append(storeCtx, dialogueID, *entry)
				}
			case EventStreamError:
				if ev.Error != nil && !recorded {
					recorded = true
					entry.Duration = time.Since(start)
					entry.Err = ev.Error.Error()
					slog.Warn("llm.audit: completion error",
						"error", ev.Error, "duration", entry.Duration)
					s.store.Append(storeCtx, dialogueID, *entry)
				}
			}
			return ev, true, nil
		},
		it.Close,
	), nil
}

func copyMessages(msgs []Message) []Message {
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}

func extractToolNames(tools []Tool) []string {
	if len(tools) == 0 {
		return nil
	}
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Function.Name
	}
	return names
}
