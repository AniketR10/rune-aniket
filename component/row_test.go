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

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRowHeight(t *testing.T) {
	t.Run("takes advantage of the full width", func(t *testing.T) {
		row := NewRow()
		row.AddComponent(NewResponsiveString("123456789", StringResponsiveConfig{}), MaxCols)
		assert.Equal(t, 1, row.Height(9))
	})
	t.Run("passes the correct width to components", func(t *testing.T) {
		row := NewRow()
		row.AddComponent(NewResponsiveString("123456789", StringResponsiveConfig{}), MaxCols/2)
		assert.Equal(t, 3, row.Height(9))
	})
	t.Run("returns the max height", func(t *testing.T) {
		row := NewRow()
		row.AddComponent(NewResponsiveString("12", StringResponsiveConfig{}), MaxCols/2)
		row.AddComponent(NewResponsiveString("123456789", StringResponsiveConfig{}), MaxCols/2)
		assert.Equal(t, 3, row.Height(9))
	})
}

func TestRowDimensions(t *testing.T) {
	row := NewRow()
	row.AddComponent(testResponsiveWidth('a', 4, 10), MaxCols/2)
	row.AddComponent(testResponsiveWidth('b', 8, 8), MaxCols/2)
	actualWidth, actualHeight := row.Dimensions()
	assert.Equal(t, 12, actualWidth)
	assert.Equal(t, 10, actualHeight)
}

func testResponsiveWidth(ch rune, wantWidth, wantHeight int) Responsive {
	return &TestResponsive{
		WantWidth:  wantWidth,
		WantHeight: wantHeight,
		TestComponent: TestComponent{
			Ch: ch,
		},
	}
}
