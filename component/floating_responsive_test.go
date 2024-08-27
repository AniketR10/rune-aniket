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
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
)

func TestFloatingResponsiveDraw(t *testing.T) {
	l := NewFloatingResponsive(NewResponsiveString("mixtral-8x7b instruct-v0.1.Q4_K_M",
		StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: StringConfig{
				Alignment: SpanAlignmentCentered,
			},
		}), 4.0/3.0)

	l.Resize(20, 4)

	w := term.NewStringWriter(20, 9)

	tests := []comptest.TestCase{
		{
			nil, `
                    
mixtral-8x7b        
instruct-v0.1.Q4_K_M
                    
                    
                    
                    
                    
                    `,
		},
		{
			func() {
				width, height := l.Dimensions()
				assert.Equal(t, 7, width)
				assert.Equal(t, 5, height)
				l.Resize(7, 5)
			}, `
mixtral             
-8x7b               
instruc             
t-v0.1.             
Q4_K_M              
                    
                    
                    
                    `,
		},
	}
	comptest.TestComponent(t, l, w, tests)

}

func TestFloatingResponsiveLoss(t *testing.T) {
	suite := []struct {
		inAspectRatio float64
		inWidth       int
		inHeight      int
		expectedLoss  float64
		expectedOk    bool
	}{
		{4.0 / 3.0, 400, 300, 0, true},
		{16.0 / 9.0, 400, 300, 0.25, false},
	}
	for _, test := range suite {
		l := NewFloatingResponsive(testResponsive('a', 10 /* nop */), test.inAspectRatio)
		actualLoss, ok := l.calculateLoss(test.inWidth, test.inHeight)
		assert.Equal(t, test.expectedLoss, actualLoss)
		assert.Equal(t, test.expectedOk, ok)
	}
}

func TestFloatingResponsiveDimensionsStatic(t *testing.T) {
	suite := []struct {
		inAspectRatio  float64
		inWidth        int
		inHeight       int
		expectedWidth  int
		expectedHeight int
	}{
		{4.0 / 3.0, 400, 300, 360, 300},
		{16.0 / 9.0, 400, 300, 480, 300},
		{4.0 / 3.0, 9, 3, 6, 3},
	}
	for _, test := range suite {
		root := testResponsiveWidth('a', test.inWidth, test.inHeight)
		l := NewFloatingResponsive(root, test.inAspectRatio)
		actualWidth, actualHeight := l.Dimensions()
		assert.Equal(t, test.expectedWidth, actualWidth)
		assert.Equal(t, test.expectedHeight, actualHeight)
	}
}

func TestFloatingResponsiveDimensionsDynamic(t *testing.T) {
	suite := []struct {
		inAspectRatio  float64
		expectedWidth  int
		expectedHeight int
		inStr          string
	}{
		{4.0 / 3.0, 7, 5, "mixtral-8x7b instruct-v0.1.Q4_K_M"},
		{4.0 / 3.0, 8, 6, "aaaaaaaaa\naaaaaaaaa\naaaaaaaaa"},
		{4.0 / 3.0, 6, 3, "a\na\na"},
		{4.0 / 3.0, 16, 12, "a\na\na\na\na\na\na\na\na\na\na\na"},
		{4.0 / 3.0, 14, 12, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{4.0 / 3.0, 36, 28, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	for _, test := range suite {
		root := NewResponsiveString(test.inStr, StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: StringConfig{
				Alignment: SpanAlignmentCentered,
			},
		})
		l := NewFloatingResponsive(root, test.inAspectRatio)
		actualWidth, actualHeight := l.Dimensions()
		assert.Equal(t, test.expectedWidth, actualWidth)
		assert.Equal(t, test.expectedHeight, actualHeight)
	}
}

func TestFloatingResponsiveIntegrationStringWithPadding(t *testing.T) {
	messageResponsive := NewResponsiveString("Do you want to restore the previous session?",
		StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: StringConfig{
				PaddingVertical:   4,
				PaddingHorizontal: 2,
				Alignment:         SpanAlignmentCentered,
			}})
	l := NewFloatingResponsive(messageResponsive, DefaultAspectRatio)
	actualWidth, actualHeight := l.Dimensions()
	assert.Equal(t, 26, actualWidth)
	assert.Equal(t, 6, actualHeight)
}

func TestFloatingResponsiveEdgeCases(t *testing.T) {

	t.Run("zero aspect ratio panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewFloatingResponsive(testResponsive('a', 10), 0)
		})
	})

	t.Run("inner component wants 0", func(t *testing.T) {
		l := NewFloatingResponsive(testResponsive('a', 0), 4.0/3.0)
		l.Resize(20, 4)
		assert.NotPanics(t, func() {
			l.Dimensions()
		})
	})

	t.Run("inner component wants large number", func(t *testing.T) {
		l := NewFloatingResponsive(testResponsive('a', 1000), 4.0/3.0)
		l.Resize(20, 4)
		width, height := l.Dimensions()
		assert.Equal(t, 1200, width)
		assert.Equal(t, 1000, height)
	})
}
