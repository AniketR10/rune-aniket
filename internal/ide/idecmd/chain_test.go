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

package idecmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChainSetGet(t *testing.T) {
	t.Parallel()
	c := NewChain()
	c.Set("VAR", "value")
	v, ok := c.Get("VAR")
	assert.True(t, ok)
	assert.Equal(t, "value", v)
	_, ok = c.Get("MISSING")
	assert.False(t, ok)
}

func TestChainNilSafe(t *testing.T) {
	t.Parallel()
	var c *Chain
	c.Set("ignored", "x") // no panic
	_, ok := c.Get("anything")
	assert.False(t, ok)
	assert.Nil(t, c.Source())
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, ok := IsContext(ctx)
	assert.False(t, ok)
	assert.Nil(t, ChainFromContext(ctx))

	chain := NewChain()
	ctx2 := WithChain(ctx, "my-alias", chain)
	name, ok := IsContext(ctx2)
	assert.True(t, ok)
	assert.Equal(t, "my-alias", name)
	assert.Same(t, chain, ChainFromContext(ctx2))
}

func TestUpdateChainVars(t *testing.T) {
	t.Parallel()
	chain := NewChain()
	ctx := WithChain(context.Background(), "a", chain)
	UpdateChainVars(ctx, map[string]string{"X": "1", "Y": "2"})
	v, ok := chain.Get("X")
	assert.True(t, ok)
	assert.Equal(t, "1", v)
	v, ok = chain.Get("Y")
	assert.True(t, ok)
	assert.Equal(t, "2", v)
}

func TestUpdateChainVarsNoChainContext(t *testing.T) {
	t.Parallel()
	// Should be a no-op (no panic).
	UpdateChainVars(context.Background(), map[string]string{"X": "1"})
}
