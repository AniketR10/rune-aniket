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

package term

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoordinates(t *testing.T) {
	suite := []struct {
		startA, endA, startB, endB Coordinates
		expectedIntersect          bool
		expectedStart              Coordinates
		expectedEnd                Coordinates
	}{
		{
			startA:            Coordinates{},
			endA:              Coordinates{},
			startB:            Coordinates{},
			endB:              Coordinates{},
			expectedIntersect: false,
		},
		{
			startA:            Coordinates{},
			endA:              Coordinates{X: 1},
			startB:            Coordinates{},
			endB:              Coordinates{X: 1},
			expectedIntersect: true,
			expectedStart:     Coordinates{},
			expectedEnd:       Coordinates{X: 1},
		},
		{
			startA:            Coordinates{},
			endA:              Coordinates{X: 2},
			startB:            Coordinates{X: 1},
			endB:              Coordinates{X: 4},
			expectedIntersect: true,
			expectedStart:     Coordinates{X: 1},
			expectedEnd:       Coordinates{X: 2},
		},
		{
			startB:            Coordinates{},
			endB:              Coordinates{X: 2},
			startA:            Coordinates{X: 1},
			endA:              Coordinates{X: 4},
			expectedIntersect: true,
			expectedStart:     Coordinates{X: 1},
			expectedEnd:       Coordinates{X: 2},
		},
		{
			endA:              Coordinates{},
			startA:            Coordinates{X: 2},
			endB:              Coordinates{X: 1},
			startB:            Coordinates{X: 4},
			expectedIntersect: true,
			expectedStart:     Coordinates{X: 1},
			expectedEnd:       Coordinates{X: 2},
		},
		{
			startA:            Coordinates{Y: 1},
			endA:              Coordinates{Y: 1},
			startB:            Coordinates{X: 1},
			endB:              Coordinates{Y: 4},
			expectedIntersect: false,
		},
		{
			startA:            Coordinates{Y: 1},
			endA:              Coordinates{Y: 1, X: 1},
			startB:            Coordinates{},
			endB:              Coordinates{Y: 4},
			expectedIntersect: true,
			expectedStart:     Coordinates{Y: 1},
			expectedEnd:       Coordinates{Y: 1, X: 1},
		},
		{
			startA:            Coordinates{},
			endA:              Coordinates{Y: 2, X: 2},
			startB:            Coordinates{Y: 1},
			endB:              Coordinates{Y: 4, X: 4},
			expectedIntersect: true,
			expectedStart:     Coordinates{Y: 1},
			expectedEnd:       Coordinates{Y: 2, X: 2},
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			actualStart, actualEnd, actualIntersect := CoordinatesIntersection(
				test.startA, test.endA, test.startB, test.endB)

			require.Equal(t, test.expectedIntersect, actualIntersect)
			assert.Equal(t, test.expectedStart, actualStart)
			assert.Equal(t, test.expectedEnd, actualEnd)
		})
	}
}
