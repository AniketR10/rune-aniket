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

func TestStoreNilSafe(t *testing.T) {
	ctx := context.Background()
	var s *store
	shown, err := s.Shown(ctx, "u", "f")
	require.NoError(t, err)
	assert.False(t, shown)
	require.NoError(t, s.MarkShown(ctx, "u", "f"))

	s = &store{}
	shown, err = s.Shown(ctx, "u", "f")
	require.NoError(t, err)
	assert.False(t, shown)
	require.NoError(t, s.MarkShown(ctx, "u", "f"))
}
