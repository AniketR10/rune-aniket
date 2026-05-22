// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package idecmd

import (
	"context"
	"sync"

	"unstable.build/go-tui/text/cmdenv"
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
