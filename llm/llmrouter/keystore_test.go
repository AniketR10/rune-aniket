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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestKeyStore_AddFirstKeyBecomesActive(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())

	require.NoError(t, s.add(ctx, ProviderOpenAI, "work", "k1"))
	active, err := s.active(ctx, ProviderOpenAI)
	require.NoError(t, err)
	assert.Equal(t, "k1", active)

	name, err := s.activeName(ctx, ProviderOpenAI)
	require.NoError(t, err)
	assert.Equal(t, "work", name)
}

func TestKeyStore_AddSecondKeyKeepsActive(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())

	require.NoError(t, s.add(ctx, ProviderOpenAI, "work", "k1"))
	require.NoError(t, s.add(ctx, ProviderOpenAI, "home", "k2"))

	active, err := s.active(ctx, ProviderOpenAI)
	require.NoError(t, err)
	assert.Equal(t, "k1", active, "adding a second key must not change the active key")
}

func TestKeyStore_Use(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())
	require.NoError(t, s.add(ctx, ProviderOpenAI, "work", "k1"))
	require.NoError(t, s.add(ctx, ProviderOpenAI, "home", "k2"))

	require.NoError(t, s.use(ctx, ProviderOpenAI, "home"))
	active, err := s.active(ctx, ProviderOpenAI)
	require.NoError(t, err)
	assert.Equal(t, "k2", active)

	err = s.use(ctx, ProviderOpenAI, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no openai api key named "missing"`)
}

func TestKeyStore_RemovePromotesActive(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())
	require.NoError(t, s.add(ctx, ProviderOpenAI, "work", "k1"))
	require.NoError(t, s.add(ctx, ProviderOpenAI, "home", "k2"))

	require.NoError(t, s.remove(ctx, ProviderOpenAI, "work"))
	active, err := s.active(ctx, ProviderOpenAI)
	require.NoError(t, err)
	assert.Equal(t, "k2", active, "removing the active key must promote a remaining key")

	require.NoError(t, s.remove(ctx, ProviderOpenAI, "missing"),
		"removing a key that does not exist must be a no-op")
}

func TestKeyStore_RemoveLastClearsActive(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())
	require.NoError(t, s.add(ctx, ProviderOpenAI, "work", "k1"))
	require.NoError(t, s.remove(ctx, ProviderOpenAI, "work"))

	_, err := s.active(ctx, ProviderOpenAI)
	assert.ErrorIs(t, err, ErrAPIKeyNotSet)
}

func TestKeyStore_Names(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())
	require.NoError(t, s.add(ctx, ProviderAnthropic, "work", "k1"))
	require.NoError(t, s.add(ctx, ProviderAnthropic, "home", "k2"))

	names, err := s.names(ctx, ProviderAnthropic)
	require.NoError(t, err)
	assert.Equal(t, []string{"home", "work"}, names)
}

func TestKeyStore_ActiveUnsetReturnsErr(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())
	_, err := s.active(ctx, ProviderGemini)
	assert.ErrorIs(t, err, ErrAPIKeyNotSet)
}

func TestKeyStore_NilStoragePanics(t *testing.T) {
	assert.PanicsWithValue(t,
		"llmrouter: newKeyStore: storage must not be nil",
		func() { newKeyStore(nil) })
}

// TestKeyStore_MutateLegacyDocWithoutVersion guards a doc written by an
// earlier build that never set a Version field. The bson marshaler omits
// the zero value, so the field is absent on disk and a Version-equality
// precondition can never match it, which previously made every mutate
// retry fail with ErrPreconditionFailed. mutate must upgrade such a doc.
func TestKeyStore_MutateLegacyDocWithoutVersion(t *testing.T) {
	ctx := context.Background()
	svc := storagestub.NewInMemoryService()
	s := newKeyStore(svc)

	type legacyKeys struct {
		Keys   map[string]string
		Active string
	}
	id := keyStoreDocID(ProviderGemini)
	require.NoError(t, svc.Create(ctx, id, &legacyKeys{
		Keys:   map[string]string{"work": "k1", "home": "k2"},
		Active: "work",
	}))

	require.NoError(t, s.remove(ctx, ProviderGemini, "work"))
	active, err := s.active(ctx, ProviderGemini)
	require.NoError(t, err)
	assert.Equal(t, "k2", active)

	require.NoError(t, s.use(ctx, ProviderGemini, "home"))
}

// TestKeyStore_VersionPreconditionIsInt64 guards the CAS type contract:
// the backing store decodes the stored Version as int64 and compares
// precondition values with Go's type-strict ==. An int-typed precondition
// must not match the stored int64 value, while an int64 one must.
func TestKeyStore_VersionPreconditionIsInt64(t *testing.T) {
	ctx := context.Background()
	svc := storagestub.NewInMemoryService()
	s := newKeyStore(svc)
	require.NoError(t, s.add(ctx, ProviderGemini, "work", "k1"))

	id := keyStoreDocID(ProviderGemini)
	var doc providerKeys
	require.NoError(t, svc.Get(ctx, id, &doc))
	noop := []storageapi.Update{{FieldPath: []string{"Active"}, Value: "work"}}

	err := svc.Update(ctx, id, noop,
		storageapi.Precondition{FieldPath: []string{"Version"}, Value: int(doc.Version)})
	require.ErrorIs(t, err, storageapi.ErrPreconditionFailed,
		"stored Version is int64; an int precondition must not match")

	require.NoError(t, svc.Update(ctx, id, noop,
		storageapi.Precondition{FieldPath: []string{"Version"}, Value: doc.Version}),
		"stored Version must satisfy an int64 precondition")
}

func TestKeyStore_ConcurrentAddsNoLostWrites(t *testing.T) {
	ctx := context.Background()
	s := newKeyStore(storagestub.NewInMemoryService())

	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			assert.NoError(t, s.add(ctx, ProviderOpenAI, fmt.Sprintf("k%02d", i), fmt.Sprintf("v%02d", i)))
		}(i)
	}
	wg.Wait()

	names, err := s.names(ctx, ProviderOpenAI)
	require.NoError(t, err)
	assert.Len(t, names, n)
}
