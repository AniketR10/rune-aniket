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

package idenotice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestStoreMarkShownRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(storagestub.NewInMemoryService())

	const uri = "file:///workspace"
	const fp = "abc123"

	shown, err := s.Shown(ctx, uri, fp)
	require.NoError(t, err)
	assert.False(t, shown)

	require.NoError(t, s.MarkShown(ctx, uri, fp))

	shown, err = s.Shown(ctx, uri, fp)
	require.NoError(t, err)
	assert.True(t, shown)

	shown, err = s.Shown(ctx, uri, "different")
	require.NoError(t, err)
	assert.False(t, shown)

	require.NoError(t, s.MarkShown(ctx, uri, "different"))
	shown, err = s.Shown(ctx, uri, fp)
	require.NoError(t, err)
	assert.False(t, shown)
	shown, err = s.Shown(ctx, uri, "different")
	require.NoError(t, err)
	assert.True(t, shown)
}
