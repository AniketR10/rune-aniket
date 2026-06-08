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

package llmrouter

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

// ErrAliasNotFound is returned when a named alias has not been set.
var ErrAliasNotFound = errors.New("llmrouter: alias not found")

const aliasStoreDocID = "aliases"

type aliasDoc struct {
	Aliases   map[string]llmapi.ModelEntry
	UpdatedAt time.Time
	Version   int64
}

type aliasStore struct {
	storage storageapi.Service
}

func newAliasStore(storage storageapi.Service) *aliasStore {
	if storage == nil {
		panic("llmrouter: newAliasStore: storage must not be nil")
	}
	return &aliasStore{storage: storage}
}

func (s *aliasStore) load(ctx context.Context) (aliasDoc, error) {
	var doc aliasDoc
	if err := s.storage.Get(ctx, aliasStoreDocID, &doc); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return aliasDoc{Aliases: map[string]llmapi.ModelEntry{}}, nil
		}
		return aliasDoc{}, fmt.Errorf("llmrouter: load aliases: %w", err)
	}
	if doc.Aliases == nil {
		doc.Aliases = map[string]llmapi.ModelEntry{}
	}
	return doc, nil
}

// mutate applies fn to the alias document under optimistic concurrency
// via storageapi.ConsistentUpdate. fn runs between the Get and the
// Update on every retry against the refreshed doc, so it must be
// idempotent. It returns true to commit or false to abort without
// writing.
func (s *aliasStore) mutate(ctx context.Context, fn func(*aliasDoc) bool) error {
	if err := s.storage.Create(ctx, aliasStoreDocID, &aliasDoc{Aliases: map[string]llmapi.ModelEntry{}, Version: 1}); err != nil &&
		!errors.Is(err, storageapi.ErrAlreadyExists) {
		return fmt.Errorf("llmrouter: create aliases: %w", err)
	}
	var doc aliasDoc
	err := storageapi.ConsistentUpdate(ctx, s.storage, aliasStoreDocID, &doc, keyStoreRetry,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if doc.Aliases == nil {
				doc.Aliases = map[string]llmapi.ModelEntry{}
			}
			version := doc.Version
			if !fn(&doc) {
				return nil, nil
			}
			updates := []storageapi.Update{
				{FieldPath: []string{"Aliases"}, Value: doc.Aliases},
				{FieldPath: []string{"Version"}, Value: version + 1},
				{FieldPath: []string{"UpdatedAt"}, Value: time.Now().UTC()},
			}
			// A zero version means the stored doc predates versioning, so a
			// Version-equality precondition can never match. Upgrade it
			// without a CAS guard; subsequent writes are versioned normally.
			if version == 0 {
				return updates, nil
			}
			return updates, []storageapi.Precondition{
				{FieldPath: []string{"Version"}, Value: version},
			}
		})
	if err != nil {
		return fmt.Errorf("llmrouter: update aliases: %w", err)
	}
	return nil
}

func (s *aliasStore) set(ctx context.Context, name string, entry llmapi.ModelEntry) error {
	if name == "" {
		return errors.New("llmrouter: alias name must not be empty")
	}
	if entry.Provider == "" || entry.Name == "" {
		return errors.New("llmrouter: alias target must be a fully-qualified model")
	}
	return s.mutate(ctx, func(doc *aliasDoc) bool {
		doc.Aliases[name] = entry
		return true
	})
}

// remove deletes a named alias. Removing an alias that does not exist
// returns ErrAliasNotFound.
func (s *aliasStore) remove(ctx context.Context, name string) error {
	var fnErr error
	if err := s.mutate(ctx, func(doc *aliasDoc) bool {
		if _, ok := doc.Aliases[name]; !ok {
			fnErr = ErrAliasNotFound
			return false
		}
		delete(doc.Aliases, name)
		return true
	}); err != nil {
		return err
	}
	return fnErr
}

func (s *aliasStore) get(ctx context.Context, name string) (llmapi.ModelEntry, bool, error) {
	doc, err := s.load(ctx)
	if err != nil {
		return llmapi.ModelEntry{}, false, err
	}
	entry, ok := doc.Aliases[name]
	return entry, ok, nil
}

func (s *aliasStore) all(ctx context.Context) (map[string]llmapi.ModelEntry, error) {
	doc, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]llmapi.ModelEntry, len(doc.Aliases))
	maps.Copy(out, doc.Aliases)
	return out, nil
}
