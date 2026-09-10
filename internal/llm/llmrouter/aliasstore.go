// Copyright (C) 2017-2026 The Rune Authors
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
