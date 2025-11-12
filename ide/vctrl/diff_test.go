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

package vctrl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/cell"
)

func TestApplyDiff(t *testing.T) {
	suite := []struct {
		src, dst string
		exp      []diffmatchpatch.Diff
	}{
		{
			src: "",
			dst: "",
			exp: []diffmatchpatch.Diff{},
		},
		{
			src: "a",
			dst: "a",
			exp: []diffmatchpatch.Diff{
				{
					Type: 0,
					Text: "a",
				},
			},
		},
		{
			src: "",
			dst: "abc\ncba",
			exp: []diffmatchpatch.Diff{
				{
					Type: 1,
					Text: "abc\ncba",
				},
			},
		},
		{
			src: "abc\ncba",
			dst: "",
			exp: []diffmatchpatch.Diff{
				{
					Type: -1,
					Text: "abc\ncba",
				},
			},
		},
		{
			src: "abc\nbcd\ncde",
			dst: "000\nabc\n111\nBCD\n",
			exp: []diffmatchpatch.Diff{
				{Type: 1, Text: "000\n"},
				{Type: 0, Text: "abc\n"},
				{Type: -1, Text: "bcd\ncde"},
				{Type: 1, Text: "111\nBCD\n"},
			},
		},
		{
			src: "A\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nÑ\nO\nP\nQ\nR\nS\nT\nU\nV\nW\nX\nY\nZ",
			dst: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\nZ",
			exp: []diffmatchpatch.Diff{
				{Type: -1, Text: "A\n"},
				{Type: 0, Text: "B\nC\nD\nE\nF\nG\n"},
				{Type: -1, Text: "H\n"},
				{Type: 0, Text: "I\nJ\nK\nL\nM\nN\n"},
				{Type: -1, Text: "Ñ\n"},
				{Type: 0, Text: "O\nP\nQ\nR\nS\nT\n"},
				{Type: -1, Text: "U\n"},
				{Type: 0, Text: "V\nW\nX\nY\nZ"},
			},
		},
		{
			src: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\nZ",
			dst: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\n",
			exp: []diffmatchpatch.Diff{
				{Type: 0, Text: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\n"},
				{Type: -1, Text: "Z"},
			},
		},
		{
			src: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\nZ",
			dst: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY",
			exp: []diffmatchpatch.Diff{
				{Type: 0, Text: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\n"},
				{Type: -1, Text: "Y\nZ"},
				{Type: 1, Text: "Y"},
			},
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			a := cell.NewBuffer()
			a.ReadFrom(strings.NewReader(test.src))
			b := cell.NewBuffer()
			b.ReadFrom(strings.NewReader(test.dst))

			ctx := context.Background()
			diff := Diff(ctx, a.String(), b.String())
			assert.Equal(t, test.exp, diff)

			ApplyDiff(ctx, a, diff)
			assert.Equal(t, b.String(), a.String())
		})
	}
}
