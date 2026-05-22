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
