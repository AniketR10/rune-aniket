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

package llmrouter

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/retry"
)

// ErrAPIKeyNotSet is returned when no active API key has been stored for
// a provider.
var ErrAPIKeyNotSet = errors.New("api key not set")

var keyStoreRetry = retry.CombinedStrategy(
	retry.ExponentialStrategy(5*time.Millisecond, 250*time.Millisecond),
	retry.LimitStrategy(10),
)

// providerKeys holds the named API keys stored for a single hosted
// provider plus which one is currently active. Regions carries per-key
// provider metadata for providers whose keys are region-scoped (Bedrock);
// it is keyed by the same names as Keys and absent for other providers.
type providerKeys struct {
	Keys      map[string]string
	Regions   map[string]string
	Active    string
	UpdatedAt time.Time
	Version   int64
}

// keyStore persists named API keys per hosted provider in rune-side
// local storage, backed by storageapi.Service.
type keyStore struct {
	storage storageapi.Service
}

func newKeyStore(storage storageapi.Service) *keyStore {
	if storage == nil {
		panic("llmrouter: newKeyStore: storage must not be nil")
	}
	return &keyStore{storage: storage}
}

func keyStoreDocID(provider string) string {
	return fmt.Sprintf("provider:%s:apikeys", provider)
}

func (s *keyStore) load(ctx context.Context, provider string) (providerKeys, error) {
	var doc providerKeys
	if err := s.storage.Get(ctx, keyStoreDocID(provider), &doc); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return providerKeys{}, nil
		}
		return providerKeys{}, fmt.Errorf("llmrouter: load %s api keys: %w", provider, err)
	}
	if doc.Keys == nil {
		doc.Keys = map[string]string{}
	}
	return doc, nil
}

// mutate applies fn to the provider's key document under optimistic
// concurrency via storageapi.ConsistentUpdate. fn runs between the Get and
// the Update on every retry against the refreshed doc, so it must be
// idempotent. It returns true to commit the change or false to abort
// without writing (e.g. when there is nothing to do, or to surface a
// validation error captured in the caller's closure).
func (s *keyStore) mutate(ctx context.Context, provider string, fn func(*providerKeys) bool) error {
	id := keyStoreDocID(provider)
	if err := s.storage.Create(ctx, id, &providerKeys{Keys: map[string]string{}, Version: 1}); err != nil &&
		!errors.Is(err, storageapi.ErrAlreadyExists) {
		return fmt.Errorf("llmrouter: create %s api keys: %w", provider, err)
	}
	var doc providerKeys
	err := storageapi.ConsistentUpdate(ctx, s.storage, id, &doc, keyStoreRetry,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if doc.Keys == nil {
				doc.Keys = map[string]string{}
			}
			if doc.Regions == nil {
				doc.Regions = map[string]string{}
			}
			version := doc.Version
			if !fn(&doc) {
				return nil, nil
			}
			updates := []storageapi.Update{
				{FieldPath: []string{"Keys"}, Value: doc.Keys},
				{FieldPath: []string{"Regions"}, Value: doc.Regions},
				{FieldPath: []string{"Active"}, Value: doc.Active},
				{FieldPath: []string{"Version"}, Value: version + 1},
				{FieldPath: []string{"UpdatedAt"}, Value: time.Now().UTC()},
			}
			// A zero version means the stored doc predates versioning (the
			// field is absent on disk, so a Version-equality precondition can
			// never match). Upgrade it without a CAS guard; subsequent writes
			// are versioned normally.
			if version == 0 {
				return updates, nil
			}
			return updates, []storageapi.Precondition{
				{FieldPath: []string{"Version"}, Value: version},
			}
		})
	if err != nil {
		return fmt.Errorf("llmrouter: update %s api keys: %w", provider, err)
	}
	return nil
}

// add stores a named key for the provider. The first key added becomes
// the active key. region records the key's scope for region-bound
// providers; pass the empty string for providers whose keys are global.
func (s *keyStore) add(ctx context.Context, provider, name, key, region string) error {
	key = strings.TrimSpace(key)
	if name == "" {
		return errors.New("llmrouter: api key name must not be empty")
	}
	if key == "" {
		return errors.New("llmrouter: api key must not be empty")
	}
	return s.mutate(ctx, provider, func(doc *providerKeys) bool {
		doc.Keys[name] = key
		if region == "" {
			delete(doc.Regions, name)
		} else {
			doc.Regions[name] = region
		}
		if doc.Active == "" {
			doc.Active = name
		}
		return true
	})
}

// remove deletes a named key. When the removed key is active, the active
// key is promoted to another remaining key (or cleared when none remain).
// Removing a key that does not exist is a no-op.
func (s *keyStore) remove(ctx context.Context, provider, name string) error {
	return s.mutate(ctx, provider, func(doc *providerKeys) bool {
		if _, ok := doc.Keys[name]; !ok {
			return false
		}
		delete(doc.Keys, name)
		delete(doc.Regions, name)
		if doc.Active == name {
			doc.Active = ""
			if remaining := sortedNames(doc.Keys); len(remaining) > 0 {
				doc.Active = remaining[0]
			}
		}
		return true
	})
}

// use sets the active key by name.
func (s *keyStore) use(ctx context.Context, provider, name string) error {
	var fnErr error
	if err := s.mutate(ctx, provider, func(doc *providerKeys) bool {
		if _, ok := doc.Keys[name]; !ok {
			fnErr = fmt.Errorf("llmrouter: no %s api key named %q", provider, name)
			return false
		}
		doc.Active = name
		return true
	}); err != nil {
		return err
	}
	return fnErr
}

// names returns the stored key names for the provider, sorted.
func (s *keyStore) names(ctx context.Context, provider string) ([]string, error) {
	doc, err := s.load(ctx, provider)
	if err != nil {
		return nil, err
	}
	return sortedNames(doc.Keys), nil
}

// active returns the value of the currently active key, or
// ErrAPIKeyNotSet when none is set.
func (s *keyStore) active(ctx context.Context, provider string) (string, error) {
	doc, err := s.load(ctx, provider)
	if err != nil {
		return "", err
	}
	if doc.Active == "" {
		return "", ErrAPIKeyNotSet
	}
	key, ok := doc.Keys[doc.Active]
	if !ok || key == "" {
		return "", ErrAPIKeyNotSet
	}
	return key, nil
}

// activeWithRegion returns the value and stored region of the currently
// active key, or ErrAPIKeyNotSet when none is set. The region is empty for
// keys stored without one.
func (s *keyStore) activeWithRegion(ctx context.Context, provider string) (string, string, error) {
	doc, err := s.load(ctx, provider)
	if err != nil {
		return "", "", err
	}
	if doc.Active == "" {
		return "", "", ErrAPIKeyNotSet
	}
	key, ok := doc.Keys[doc.Active]
	if !ok || key == "" {
		return "", "", ErrAPIKeyNotSet
	}
	return key, doc.Regions[doc.Active], nil
}

// regions returns the stored key-name -> region mapping for the provider.
// Keys stored without a region are absent from the map.
func (s *keyStore) regions(ctx context.Context, provider string) (map[string]string, error) {
	doc, err := s.load(ctx, provider)
	if err != nil {
		return nil, err
	}
	return doc.Regions, nil
}

// activeName returns the name of the currently active key, or the empty
// string when none is set.
func (s *keyStore) activeName(ctx context.Context, provider string) (string, error) {
	doc, err := s.load(ctx, provider)
	if err != nil {
		return "", err
	}
	return doc.Active, nil
}

func (s *keyStore) activeUpdatedAt(ctx context.Context, provider string) (time.Time, bool, error) {
	doc, err := s.load(ctx, provider)
	if err != nil {
		return time.Time{}, false, err
	}
	if doc.Active == "" {
		return time.Time{}, false, nil
	}
	if key, ok := doc.Keys[doc.Active]; !ok || key == "" {
		return time.Time{}, false, nil
	}
	return doc.UpdatedAt, true, nil
}

func sortedNames(keys map[string]string) []string {
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
