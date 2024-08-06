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
	"testing"

	"github.com/stretchr/testify/assert"
)

// this test is not very thorough because StringWriter is used by
// many tui.Component tests and so it's already indirectly tested.
func TestStringWriter(t *testing.T) {
	width, height := 5, 6
	writer := NewStringWriter(width, height)

	c := 'A'
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			if i > j-1 {
				writer.SetCell(Coordinates{X: j, Y: i}, Cell{Ch: c})
			}
		}
		c++
	}

	// we use StringWriter mostly for tests so it's important that it panics on oob
	assert.Panics(t, func() {
		writer.SetCell(Coordinates{X: width + 1, Y: height + 1}, Cell{Ch: '='})
	})

	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := "A    \nBB   \nCCC  \nDDDD \nEEEEE\n     "
	assert.Equal(t, expected, writer.String())
}

// TODO
// func TestRuneLength(t *testing.T) {
// 	width, height := 5, 6
// 	writer := NewStringWriter(width, height)
//
// 	c := '中'
// 	for i := 0; i < width; i++ {
// 		for j := 0; j < height; j++ {
// 			writer.SetCell(Coordinates{X: j, Y: i}, Cell{Ch: c})
// 		}
// 		c++
// 	}
//
// 	if err := writer.Flush(); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	expected := "中中 \n丮丮 \n丯丯 \n丰丰 \n丱丱 \n     "
// 	if writer.String() != expected {
// 		t.Errorf("expected: %q; found: %q", expected, writer.String())
// 	}
// }
