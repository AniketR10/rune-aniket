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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

func testString(t *testing.T,
	fn func(string) tui.Component,
	width, height int, in, out string,
) {
	w := term.NewStringWriter(width, height)
	comp := fn(in)
	comp.Resize(width, height)
	comp.Draw(w)
	require.NoError(t, w.Flush())
	assert.Equal(t, out, w.String())
}

func TestStringCentered(t *testing.T) {
	tcases := []struct {
		in  string
		out string
	}{
		{
			in:  "aaaa",
			out: "     \n     \naaaa \n     \n     ",
		},
		{
			in:  "XXXXXXXXXXXXXXXX\nXXXXXXXXXX\nXXXXXXXXXX",
			out: "     \nXXXXX\nXXXXX\nXXXXX\n     ",
		},
		{
			in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
			out: "  X  \n  X  \n  X  \n  X  \n  X  ",
		},
		{
			in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
			out: "  X  \n  X  \n  X  \n  X  \n  X  ",
		},
		{
			in:  "XXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\n",
			out: "XXXXX\nXXXXX\nXXXXX\nXXXXX\nXXXXX",
		},
		{
			in:  "a",
			out: "     \n     \n  a  \n     \n     ",
		},
	}

	for _, tcase := range tcases {
		testString(t, func(str string) tui.Component {
			return NewStringWithConfig(str, StringConfig{Alignment: SpanAlignmentCentered})
		}, 5, 5, tcase.in, tcase.out)
	}
}

func TestStringWithConfigDimensions(t *testing.T) {
	tcases := []struct {
		in             string
		expectedWidth  int
		expectedHeight int
	}{
		{
			in:             "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF",
			expectedWidth:  12,
			expectedHeight: 6,
		},
		{
			in:             "X",
			expectedWidth:  1,
			expectedHeight: 1,
		},
		{
			in:             "X\nX\nX\nX\n",
			expectedWidth:  1,
			expectedHeight: 5,
		},
		{
			in:             "XXXXXX\nXXXX\nXXX\nXX\n",
			expectedWidth:  6,
			expectedHeight: 5,
		},
		{
			in:             "XXXXXXX\nXXXX\nXX\nXXXXXXXXXX",
			expectedWidth:  10,
			expectedHeight: 4,
		},
	}

	for i, tcase := range tcases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			s := NewStringWithConfig(tcase.in, StringConfig{})
			actualWidth, actualHeight := s.Dimensions()
			assert.Equal(t, tcase.expectedWidth, actualWidth)
			assert.Equal(t, tcase.expectedHeight, actualHeight)
		})
	}
}

func benchmarkString(b *testing.B, fortunes int) {
	var builder strings.Builder
	for i := 0; i < fortunes; i++ {
		_, _ = builder.WriteString(fortune)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewString(builder.String())
	}
}

func BenchmarkString1(b *testing.B) {
	benchmarkString(b, 1)
}
func BenchmarkString10(b *testing.B) {
	benchmarkString(b, 10)
}
func BenchmarkString100(b *testing.B) {
	benchmarkString(b, 100)
}
func BenchmarkString1000(b *testing.B) {
	benchmarkString(b, 1000)
}
func BenchmarkString10000(b *testing.B) {
	benchmarkString(b, 10000)
}
