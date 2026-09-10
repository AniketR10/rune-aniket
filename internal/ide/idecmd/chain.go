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

package idecmd

import (
	"context"
	"sync"

	"unstable.build/rune/internal/text/cmdenv"
)

type ctxKey int

var chainKey ctxKey

type chainContext struct {
	name  string
	chain *Chain
}

// Chain captures variable assignments produced by `!!` steps inside
// one alias invocation so subsequent steps can resolve $NAME against
// them. It is concurrency-safe.
type Chain struct {
	mu   sync.RWMutex
	vars map[string]string
}

// NewChain returns an empty Chain ready for use.
func NewChain() *Chain {
	return &Chain{vars: make(map[string]string)}
}

// Set records name=value in the chain.
func (c *Chain) Set(name, value string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vars == nil {
		c.vars = make(map[string]string)
	}
	c.vars[name] = value
}

// Get returns the captured value for name and true, or "" and false.
func (c *Chain) Get(name string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.vars[name]
	return v, ok
}

// Source adapts the chain to cmdenv.Source.
func (c *Chain) Source() cmdenv.Source {
	if c == nil {
		return nil
	}
	return c.Get
}

// IsContext reports whether ctx was produced by withChain — i.e. the
// dispatch is happening inside an alias expansion. Command handlers
// can call it to detect alias-dispatched invocations.
func IsContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(chainKey).(chainContext)
	if !ok {
		return "", false
	}
	return v.name, true
}

// ChainFromContext returns the active chain attached to ctx, or nil
// when ctx is not inside an alias dispatch.
func ChainFromContext(ctx context.Context) *Chain {
	v, _ := ctx.Value(chainKey).(chainContext)
	return v.chain
}

// UpdateChainVars stores name/value pairs in the alias chain attached
// to ctx so subsequent alias steps can expand them as $NAME. No-op
// when ctx is not inside an alias dispatch.
func UpdateChainVars(ctx context.Context, vars map[string]string) {
	chain := ChainFromContext(ctx)
	if chain == nil {
		return
	}
	for name, value := range vars {
		chain.Set(name, value)
	}
}

// WithChain attaches a chain to ctx scoped to the named alias. Leaf
// handlers dispatched under the returned ctx can publish capture
// vars via UpdateChainVars that subsequent alias-step expansions
// will see.
func WithChain(ctx context.Context, name string, c *Chain) context.Context {
	return context.WithValue(ctx, chainKey, chainContext{name: name, chain: c})
}
