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

package drawrect

import (
	"image/color"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/benchdraw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBatchAccumulatesRects asserts many rectangles accumulate into a
// single vertex/index buffer with correctly offset indices and per-quad
// colors, and flush as one DrawTriangles call.
func TestBatchAccumulatesRects(t *testing.T) {
	Init()
	var b Batch
	assert.True(t, b.Empty())

	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	b.AddRect(0, 0, 10, 10, red)
	b.AddRect(20, 0, 10, 10, blue)
	require.False(t, b.Empty())

	// Two filled quads: 4 vertices and 6 indices (two triangles) each.
	assert.Len(t, b.vertices, 8)
	assert.Len(t, b.indices, 12)
	// Per-quad colors are baked into the vertices.
	assert.Equal(t, float32(1), b.vertices[0].ColorR)
	assert.Equal(t, float32(0), b.vertices[0].ColorB)
	assert.Equal(t, float32(1), b.vertices[4].ColorB)
	assert.Equal(t, float32(0), b.vertices[4].ColorR)
	// The second quad's six indices are offset past the first quad's
	// four vertices, so both triangles reference the correct vertices.
	for _, idx := range b.indices[6:] {
		assert.GreaterOrEqual(t, idx, uint16(4))
	}

	// One Flush of a non-empty batch issues exactly one DrawTriangles by
	// construction and then clears the batch.
	dst := ebiten.NewImage(64, 16)
	benchdraw.BeginFrame(t)
	b.Flush(dst)
	benchdraw.EndFrame(t)
	assert.True(t, b.Empty(), "flush resets the batch")
}

// TestBatchFlushEmptyNoOp asserts flushing an empty batch does nothing.
func TestBatchFlushEmptyNoOp(t *testing.T) {
	Init()
	var b Batch
	dst := ebiten.NewImage(8, 8)
	require.True(t, b.Empty())
	benchdraw.BeginFrame(t)
	b.Flush(dst)
	benchdraw.EndFrame(t)
	assert.True(t, b.Empty(), "flushing an empty batch is a no-op")
}
