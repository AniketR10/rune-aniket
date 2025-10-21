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

func TestCoordinatesMerging(t *testing.T) {
	t.Run("two ranges", func(t *testing.T) {
		suite := []struct {
			name                       string
			startA, endA, startB, endB Coordinates
			expected                   []Range
		}{
			{
				name:     "identical zero ranges",
				startA:   Coordinates{},
				endA:     Coordinates{},
				startB:   Coordinates{},
				endB:     Coordinates{},
				expected: []Range{{}},
			},
			{
				name:     "identical ranges on same line",
				startA:   Coordinates{},
				endA:     Coordinates{X: 1},
				startB:   Coordinates{},
				endB:     Coordinates{X: 1},
				expected: []Range{{Start: Coordinates{}, End: Coordinates{X: 1}}},
			},
			{
				name:     "overlapping ranges on same line",
				startA:   Coordinates{},
				endA:     Coordinates{X: 2},
				startB:   Coordinates{X: 1},
				endB:     Coordinates{X: 4},
				expected: []Range{{Start: Coordinates{}, End: Coordinates{X: 4}}},
			},
			{
				name:     "overlapping ranges on same line (reversed)",
				startB:   Coordinates{},
				endB:     Coordinates{X: 2},
				startA:   Coordinates{X: 1},
				endA:     Coordinates{X: 4},
				expected: []Range{{Start: Coordinates{}, End: Coordinates{X: 4}}},
			},
			{
				name:     "overlapping ranges with reversed coordinates",
				endA:     Coordinates{},
				startA:   Coordinates{X: 2},
				endB:     Coordinates{X: 1},
				startB:   Coordinates{X: 4},
				expected: []Range{{Start: Coordinates{}, End: Coordinates{X: 4}}},
			},
			{
				name:     "multi-line and single-line merge",
				startA:   Coordinates{Y: 1},
				endA:     Coordinates{Y: 1},
				startB:   Coordinates{X: 1},
				endB:     Coordinates{Y: 4},
				expected: []Range{{Start: Coordinates{X: 1}, End: Coordinates{Y: 4}}},
			},
			{
				name:     "range spanning multiple lines",
				startA:   Coordinates{Y: 1},
				endA:     Coordinates{Y: 1, X: 1},
				startB:   Coordinates{},
				endB:     Coordinates{Y: 4},
				expected: []Range{{Start: Coordinates{}, End: Coordinates{Y: 4}}},
			},
			{
				name:     "overlapping multi-line ranges",
				startA:   Coordinates{},
				endA:     Coordinates{Y: 2, X: 2},
				startB:   Coordinates{Y: 1},
				endB:     Coordinates{Y: 4, X: 4},
				expected: []Range{{Start: Coordinates{}, End: Coordinates{Y: 4, X: 4}}},
			},
			{
				name:     "adjacent ranges on same line",
				startA:   Coordinates{X: 0},
				endA:     Coordinates{X: 5},
				startB:   Coordinates{X: 5},
				endB:     Coordinates{X: 10},
				expected: []Range{{Start: Coordinates{X: 0}, End: Coordinates{X: 10}}},
			},
			{
				name:     "non-overlapping ranges on same line",
				startA:   Coordinates{X: 0},
				endA:     Coordinates{X: 5},
				startB:   Coordinates{X: 6},
				endB:     Coordinates{X: 10},
				expected: []Range{{Start: Coordinates{X: 0}, End: Coordinates{X: 5}}, {Start: Coordinates{X: 6}, End: Coordinates{X: 10}}},
			},
			{
				name:     "range A completely contains range B",
				startA:   Coordinates{Y: 0},
				endA:     Coordinates{Y: 10},
				startB:   Coordinates{Y: 3},
				endB:     Coordinates{Y: 5},
				expected: []Range{{Start: Coordinates{Y: 0}, End: Coordinates{Y: 10}}},
			},
			{
				name:     "range B completely contains range A",
				startA:   Coordinates{Y: 3},
				endA:     Coordinates{Y: 5},
				startB:   Coordinates{Y: 0},
				endB:     Coordinates{Y: 10},
				expected: []Range{{Start: Coordinates{Y: 0}, End: Coordinates{Y: 10}}},
			},
			{
				name:     "adjacent ranges across lines",
				startA:   Coordinates{Y: 0},
				endA:     Coordinates{Y: 2, X: 10},
				startB:   Coordinates{Y: 2, X: 10},
				endB:     Coordinates{Y: 5},
				expected: []Range{{Start: Coordinates{Y: 0}, End: Coordinates{Y: 5}}},
			},
			{
				name:     "non-overlapping ranges on different lines",
				startA:   Coordinates{Y: 0},
				endA:     Coordinates{Y: 2},
				startB:   Coordinates{Y: 5},
				endB:     Coordinates{Y: 8},
				expected: []Range{{Start: Coordinates{Y: 0}, End: Coordinates{Y: 2}}, {Start: Coordinates{Y: 5}, End: Coordinates{Y: 8}}},
			},
			{
				name:     "single character range",
				startA:   Coordinates{Y: 1, X: 5},
				endA:     Coordinates{Y: 1, X: 5},
				startB:   Coordinates{Y: 1, X: 5},
				endB:     Coordinates{Y: 1, X: 5},
				expected: []Range{{Start: Coordinates{Y: 1, X: 5}, End: Coordinates{Y: 1, X: 5}}},
			},
			{
				name:     "overlapping single character with range",
				startA:   Coordinates{Y: 1, X: 5},
				endA:     Coordinates{Y: 1, X: 5},
				startB:   Coordinates{Y: 1, X: 3},
				endB:     Coordinates{Y: 1, X: 7},
				expected: []Range{{Start: Coordinates{Y: 1, X: 3}, End: Coordinates{Y: 1, X: 7}}},
			},
			{
				name:     "ranges touching at line boundary",
				startA:   Coordinates{Y: 1},
				endA:     Coordinates{Y: 3},
				startB:   Coordinates{Y: 3},
				endB:     Coordinates{Y: 5},
				expected: []Range{{Start: Coordinates{Y: 1}, End: Coordinates{Y: 5}}},
			},
			{
				name:     "partial line overlap",
				startA:   Coordinates{Y: 2, X: 5},
				endA:     Coordinates{Y: 4, X: 10},
				startB:   Coordinates{Y: 3, X: 2},
				endB:     Coordinates{Y: 5, X: 3},
				expected: []Range{{Start: Coordinates{Y: 2, X: 5}, End: Coordinates{Y: 5, X: 3}}},
			},
			{
				name:     "same start different ends",
				startA:   Coordinates{Y: 1, X: 0},
				endA:     Coordinates{Y: 2, X: 0},
				startB:   Coordinates{Y: 1, X: 0},
				endB:     Coordinates{Y: 3, X: 0},
				expected: []Range{{Start: Coordinates{Y: 1, X: 0}, End: Coordinates{Y: 3, X: 0}}},
			},
			{
				name:     "same end different starts",
				startA:   Coordinates{Y: 1, X: 0},
				endA:     Coordinates{Y: 3, X: 0},
				startB:   Coordinates{Y: 2, X: 0},
				endB:     Coordinates{Y: 3, X: 0},
				expected: []Range{{Start: Coordinates{Y: 1, X: 0}, End: Coordinates{Y: 3, X: 0}}},
			},
			{
				name:     "zero-width range at line start",
				startA:   Coordinates{Y: 5},
				endA:     Coordinates{Y: 5},
				startB:   Coordinates{Y: 4, X: 10},
				endB:     Coordinates{Y: 6, X: 5},
				expected: []Range{{Start: Coordinates{Y: 4, X: 10}, End: Coordinates{Y: 6, X: 5}}},
			},
			{
				name:     "overlapping at exact column on same line",
				startA:   Coordinates{Y: 5, X: 10},
				endA:     Coordinates{Y: 5, X: 20},
				startB:   Coordinates{Y: 5, X: 15},
				endB:     Coordinates{Y: 5, X: 25},
				expected: []Range{{Start: Coordinates{Y: 5, X: 10}, End: Coordinates{Y: 5, X: 25}}},
			},
		}
		for _, test := range suite {
			t.Run(test.name, func(t *testing.T) {
				actual := MergeRanges(
					[]Range{{test.startA, test.endA}, {test.startB, test.endB}})
				assert.Equal(t, test.expected, actual)
			})
		}
	})

	t.Run("multiple ranges", func(t *testing.T) {
		suite := []struct {
			name     string
			ranges   []Range
			expected []Range
		}{
			{
				name:     "empty input",
				ranges:   []Range{},
				expected: []Range{},
			},
			{
				name:     "single range",
				ranges:   []Range{{Start: Coordinates{Y: 1}, End: Coordinates{Y: 3}}},
				expected: []Range{{Start: Coordinates{Y: 1}, End: Coordinates{Y: 3}}},
			},
			{
				name: "three overlapping ranges",
				ranges: []Range{
					{Start: Coordinates{Y: 1}, End: Coordinates{Y: 3}},
					{Start: Coordinates{Y: 2}, End: Coordinates{Y: 4}},
					{Start: Coordinates{Y: 3, X: 5}, End: Coordinates{Y: 5}},
				},
				expected: []Range{{Start: Coordinates{Y: 1}, End: Coordinates{Y: 5}}},
			},
			{
				name: "two separate groups",
				ranges: []Range{
					{Start: Coordinates{Y: 1}, End: Coordinates{Y: 2}},
					{Start: Coordinates{Y: 1, X: 5}, End: Coordinates{Y: 3}},
					{Start: Coordinates{Y: 10}, End: Coordinates{Y: 11}},
					{Start: Coordinates{Y: 10, X: 5}, End: Coordinates{Y: 12}},
				},
				expected: []Range{
					{Start: Coordinates{Y: 1}, End: Coordinates{Y: 3}},
					{Start: Coordinates{Y: 10}, End: Coordinates{Y: 12}},
				},
			},
			{
				name: "unsorted input",
				ranges: []Range{
					{Start: Coordinates{Y: 5}, End: Coordinates{Y: 7}},
					{Start: Coordinates{Y: 1}, End: Coordinates{Y: 2}},
					{Start: Coordinates{Y: 3}, End: Coordinates{Y: 4}},
				},
				expected: []Range{
					{Start: Coordinates{Y: 1}, End: Coordinates{Y: 2}},
					{Start: Coordinates{Y: 3}, End: Coordinates{Y: 4}},
					{Start: Coordinates{Y: 5}, End: Coordinates{Y: 7}},
				},
			},
			{
				name: "all ranges merge into one",
				ranges: []Range{
					{Start: Coordinates{Y: 0}, End: Coordinates{Y: 2}},
					{Start: Coordinates{Y: 1}, End: Coordinates{Y: 3}},
					{Start: Coordinates{Y: 2}, End: Coordinates{Y: 4}},
					{Start: Coordinates{Y: 3}, End: Coordinates{Y: 5}},
				},
				expected: []Range{{Start: Coordinates{Y: 0}, End: Coordinates{Y: 5}}},
			},
			{
				name: "nested ranges",
				ranges: []Range{
					{Start: Coordinates{Y: 0}, End: Coordinates{Y: 10}},
					{Start: Coordinates{Y: 2}, End: Coordinates{Y: 3}},
					{Start: Coordinates{Y: 5}, End: Coordinates{Y: 6}},
					{Start: Coordinates{Y: 8}, End: Coordinates{Y: 9}},
				},
				expected: []Range{{Start: Coordinates{Y: 0}, End: Coordinates{Y: 10}}},
			},
			{
				name: "duplicate ranges",
				ranges: []Range{
					{Start: Coordinates{Y: 1, X: 5}, End: Coordinates{Y: 3, X: 10}},
					{Start: Coordinates{Y: 1, X: 5}, End: Coordinates{Y: 3, X: 10}},
					{Start: Coordinates{Y: 1, X: 5}, End: Coordinates{Y: 3, X: 10}},
				},
				expected: []Range{{Start: Coordinates{Y: 1, X: 5}, End: Coordinates{Y: 3, X: 10}}},
			},
		}
		for _, test := range suite {
			t.Run(test.name, func(t *testing.T) {
				actual := MergeRanges(test.ranges)
				assert.Equal(t, test.expected, actual)
			})
		}
	})
}

func TestCoordinatesIntersection(t *testing.T) {
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
