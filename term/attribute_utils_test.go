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
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
)

func TestAttributesUnion(t *testing.T) {
	tsuite := []struct {
		inA, inB Attributes
		wantOut  Attributes
	}{
		{},

		{Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorDefault}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorDefault}, Attributes{Bg: tcell.ColorRed}},

		{Attributes{Fg: tcell.ColorWhite}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorWhite}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorRed}},

		{Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse},
			Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse},
			Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse}},
		{Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse},
			Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse},
			Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse}},

		{Attributes{Attrs: tcell.AttrBold}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold}},
		{Attributes{Attrs: tcell.AttrBold}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold}},
		{Attributes{Attrs: tcell.AttrUnderline}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrUnderline}},
		{Attributes{Attrs: tcell.AttrUnderline}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrUnderline}},
		{Attributes{Attrs: tcell.AttrReverse}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrReverse}},
		{Attributes{Attrs: tcell.AttrReverse}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrReverse}},

		{Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrBold}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrBold}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold}},
		{Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrUnderline}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrUnderline}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrUnderline}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrUnderline}},
		{Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrReverse}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrReverse}},

		{Attributes{Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline},
			Attributes{Bg: tcell.ColorRed},
			Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline}},
		{Attributes{Bg: tcell.ColorRed},
			Attributes{Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline},
			Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline}},

		{Attributes{Fg: tcell.ColorDefault}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorDefault}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorRed}},
	}

	for i, tcase := range tsuite {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actualOut := AttributesUnion(tcase.inA, tcase.inB)
			assert.Equal(t, tcase.wantOut, actualOut)
		})
	}
}

func TestAttributesDifference(t *testing.T) {
	tsuite := []struct {
		inA, inB Attributes
		wantOut  Attributes
	}{
		{},

		{Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse}, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse},
			Attributes{Fg: tcell.ColorDefault}},
		{Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse}, Attributes{Bg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline | tcell.AttrReverse},
			Attributes{Bg: tcell.ColorDefault}},

		{Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorDefault}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorDefault}, Attributes{Bg: tcell.ColorRed}},

		{Attributes{Fg: tcell.ColorWhite}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorWhite}},
		{Attributes{Bg: tcell.ColorWhite}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorWhite}},

		{Attributes{Fg: tcell.ColorWhite}, Attributes{Fg: tcell.ColorWhite}, Attributes{Fg: tcell.ColorDefault}},
		{Attributes{Bg: tcell.ColorWhite}, Attributes{Bg: tcell.ColorWhite}, Attributes{Bg: tcell.ColorDefault}},

		{Attributes{Attrs: tcell.AttrBold}, Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrBold}},
		{Attributes{Attrs: tcell.AttrBold}, Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrBold}},
		{Attributes{Attrs: tcell.AttrUnderline}, Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrUnderline}},
		{Attributes{Attrs: tcell.AttrUnderline}, Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrUnderline}},
		{Attributes{Attrs: tcell.AttrReverse}, Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse}},
		{Attributes{Attrs: tcell.AttrReverse}, Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse}},

		{Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrBold}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrBold}, Attributes{Bg: tcell.ColorRed}},
		{Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrUnderline}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrUnderline}, Attributes{Bg: tcell.ColorRed}},
		{Attributes{Fg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse}, Attributes{Fg: tcell.ColorRed}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse}, Attributes{Bg: tcell.ColorRed}},

		{Attributes{Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline}, Attributes{Bg: tcell.ColorRed},
			Attributes{Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline}},
		{Attributes{Bg: tcell.ColorRed}, Attributes{Attrs: tcell.AttrReverse | tcell.AttrBold | tcell.AttrUnderline},
			Attributes{Bg: tcell.ColorRed}},

		{Attributes{Fg: tcell.ColorDefault}, Attributes{Fg: tcell.ColorRed}, Attributes{Fg: tcell.ColorDefault}},
		{Attributes{Bg: tcell.ColorDefault}, Attributes{Bg: tcell.ColorRed}, Attributes{Bg: tcell.ColorDefault}},
	}

	for i, tcase := range tsuite {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actualOut := AttributesDifference(tcase.inA, tcase.inB)
			assert.Equal(t, tcase.wantOut, actualOut)
		})
	}
}

func BenchmarkAttributesOperations(b *testing.B) {
	attr := Attributes{Fg: tcell.ColorDefault, Bg: tcell.ColorDefault}
	for i := 0; i < b.N; i++ {
		attr = AttributesUnion(attr, Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold | tcell.AttrUnderline})
		attr = AttributesDifference(attr, Attributes{Attrs: tcell.AttrBold | tcell.AttrReverse})
		attr = AttributesUnion(attr, Attributes{Fg: tcell.ColorGreen, Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack})
	}
}
