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
