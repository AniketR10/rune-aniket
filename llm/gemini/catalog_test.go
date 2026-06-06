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

package gemini

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// stubIterator yields entries then ends with an optional error.
type stubIterator struct {
	entries []llmapi.ModelEntry
	i       int
	err     error
}

func (s *stubIterator) Next(context.Context) (llmapi.ModelEntry, bool) {
	if s.i >= len(s.entries) {
		return llmapi.ModelEntry{}, false
	}
	e := s.entries[s.i]
	s.i++
	return e, true
}
func (s *stubIterator) Err() error   { return s.err }
func (s *stubIterator) Close() error { return nil }

func drainCatalog(t *testing.T, it iterator.Iterator[llmapi.ModelEntry]) []llmapi.ModelEntry {
	t.Helper()
	var out []llmapi.ModelEntry
	for {
		e, ok := it.Next(context.Background())
		if !ok {
			break
		}
		out = append(out, e)
	}
	return out
}

func TestFallbackIterator(t *testing.T) {
	fallback := []llmapi.ModelEntry{{Name: "static-a"}, {Name: "static-b"}}
	liveErr := errors.New("permission denied")
	live := []llmapi.ModelEntry{{Name: "live-a"}}

	t.Run("live success serves live, no error", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{entries: live}, fallback)
		got := drainCatalog(t, it)
		assert.Equal(t, live, got)
		require.NoError(t, it.Err())
	})

	t.Run("error before any entry serves fallback and surfaces error", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{err: liveErr}, fallback)
		got := drainCatalog(t, it)
		assert.Equal(t, fallback, got)
		require.ErrorIs(t, it.Err(), liveErr)
	})

	t.Run("successful empty list stays empty", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{}, fallback)
		got := drainCatalog(t, it)
		assert.Empty(t, got)
		require.NoError(t, it.Err())
	})

	t.Run("error after partial yield keeps partial and surfaces error", func(t *testing.T) {
		it := withStaticFallback(&stubIterator{entries: live, err: liveErr}, fallback)
		got := drainCatalog(t, it)
		assert.Equal(t, live, got, "must not append the fallback to partial live results")
		require.ErrorIs(t, it.Err(), liveErr)
	})
}
