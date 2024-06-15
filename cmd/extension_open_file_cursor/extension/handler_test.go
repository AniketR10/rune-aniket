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
package extension

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

func TestWordURI(t *testing.T) {
	var content = `Love in your heart wasn't put there to stay.
file://tato+a:pass@redbaycoffee.com/tmp/a
`
	tsuite := []struct {
		in      term.Coordinates
		wantOut string
	}{
		{term.Coordinates{}, "Love"},
		{term.Coordinates{X: 4}, ""},
		{term.Coordinates{X: 6}, "in"},
		{term.Coordinates{Y: 1, X: 0}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 4}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 5}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 6}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 7}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 17}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 33}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d: %v", i, tcase.in), func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.WriteString(content)

			var f testResource
			f.Scroll.Init(buf)
			f.cursor.Init(&f.Scroll)
			f.cursor.MoveToScroll(tcase.in)
			assert.Equal(t, tcase.wantOut, uriAtCursor(&f))
		})
	}
}

type testResource struct {
	component.Scroll
	cursor text.Cursor
}

func (t *testResource) Cursor() term.Coordinates {
	return t.cursor.CursorAtScroll()
}
