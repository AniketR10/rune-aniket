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
)

func TestLoadDefaultFont(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		assert.NotNil(t, m.RegularFontFace())
	})
}

func TestPixelAndCellCalculation(t *testing.T) {
	t.Run("doesn't panic", func(t *testing.T) {
		m, err := NewManager(100, 2)
		require.NoError(t, err)
		assert.NotPanics(t, func() {
			m.PixelX(-10)
			m.PixelY(-10)
			m.CellX(-10)
			m.CellY(-10)
		})
	})

	t.Run("PixelX/Y returns the pixel corresponding to the given cell", func(t *testing.T) {
		m, err := NewManager(2, 1)
		require.NoError(t, err)
		assert.NotNil(t, m.RegularFontFace())
		assert.Equal(t, float64(190), m.PixelX(10))
		assert.Equal(t, float64(420), m.PixelY(10))
	})

	t.Run("CellX/Y returns the cell corresponding to the given pixel", func(t *testing.T) {
		m, err := NewManager(2, 1)
		require.NoError(t, err)
		assert.NotNil(t, m.RegularFontFace())
		assert.Equal(t, 10, m.CellX(190))
		assert.Equal(t, 10, m.CellY(420))
	})
}
