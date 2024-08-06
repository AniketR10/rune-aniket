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

package font

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/math/fixed"
)

func TestMultiFace(t *testing.T) {
	t.Run("Glyph keeps calling Glyph if faces return false, returns preferred", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		face.Glyph(fixed.Point26_6{}, 'a')
		assert.Equal(t, 2, mock1.glyph)
		assert.Equal(t, 1, mock2.glyph)
	})

	t.Run("GlyphBouns keeps calling GlyphBounds if faces return false, returns preferred", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		face.GlyphBounds('a')
		assert.Equal(t, 2, mock1.bounds)
		assert.Equal(t, 1, mock2.bounds)
	})

	t.Run("GlyphAdvance keeps calling GlyphAdvance if faces return false, returns preferred", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		face.GlyphAdvance('a')
		assert.Equal(t, 2, mock1.advance)
		assert.Equal(t, 1, mock2.advance)
	})

	t.Run("Kern calls preferred face", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(1, &mock1, &mock2)

		face.Kern('a', 'b')
		assert.Equal(t, 0, mock1.kern)
		assert.Equal(t, 1, mock2.kern)
	})

	t.Run("Metrics calls preferred face", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(1, &mock1, &mock2)

		face.Metrics()
		assert.Equal(t, 0, mock1.metrics)
		assert.Equal(t, 1, mock2.metrics)
	})

	t.Run("Close dispatches Close to all underlying faces", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		require.NoError(t, face.Close())
		assert.Equal(t, 1, mock1.close)
		assert.Equal(t, 1, mock2.close)
	})
}
