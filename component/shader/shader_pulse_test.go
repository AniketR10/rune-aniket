// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package shader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// TestPulseLeavesCycleBoundariesUntouched asserts that Pulse does not
// mutate cells at cycle boundaries (frame=0, frame=PeriodFrames, ...).
func TestPulseLeavesCycleBoundariesUntouched(t *testing.T) {
	t.Parallel()
	defaultAttr := term.Attributes{Fg: term.ColorWhite, Bg: term.ColorBlack}
	in := [][]term.Cell{{
		{Ch: 'x', Attributes: term.Attributes{Fg: term.NewRGBColor(10, 20, 30), Bg: term.NewRGBColor(40, 50, 60)}},
	}}
	want := [][]term.Cell{{
		{Ch: 'x', Attributes: term.Attributes{Fg: term.NewRGBColor(10, 20, 30), Bg: term.NewRGBColor(40, 50, 60)}},
	}}

	params := DefaultPulseParams()
	sh := Pulse(params, defaultAttr)
	sh.Shade(0, 1000, in)
	assert.Equal(t, want, in, "Pulse must not mutate cells at frame=0")

	sh.Shade(params.PeriodFrames, 1000, in)
	assert.Equal(t, want, in,
		"Pulse must not mutate cells at cycle boundary "+
			"(frame == PeriodFrames)")
}

// TestPulsePeakBlendsTowardColor asserts that mid-cycle Pulse blends
// Fg toward Color.
func TestPulsePeakBlendsTowardColor(t *testing.T) {
	t.Parallel()
	defaultAttr := term.Attributes{Fg: term.ColorWhite, Bg: term.ColorBlack}
	in := [][]term.Cell{{
		{Ch: 'x', Attributes: term.Attributes{
			Fg: term.NewRGBColor(10, 10, 10),
			Bg: term.NewRGBColor(20, 20, 20),
		}},
	}}
	params := DefaultPulseParams()
	params.Color = term.NewRGBColor(255, 0, 0)
	params.Intensity = 1.0
	Pulse(params, defaultAttr).Shade(params.PeriodFrames/2, 1000, in)

	got := uint32(in[0][0].Fg) & 0x00FFFFFF
	r := int(got >> 16 & 0xff)
	assert.Greater(t, r, 10,
		"at peak intensity Fg.R should move toward Color.R")
}

// TestPulseSkipsCellsOutsideAnimation asserts that a negative frame is
// ignored and does not panic.
func TestPulseSkipsCellsOutsideAnimation(t *testing.T) {
	t.Parallel()
	in := [][]term.Cell{{
		{Ch: 'x', Attributes: term.Attributes{Fg: term.ColorRed}},
	}}
	want := [][]term.Cell{{
		{Ch: 'x', Attributes: term.Attributes{Fg: term.ColorRed}},
	}}
	Pulse(DefaultPulseParams(), term.Attributes{}).Shade(-1, 1000, in)
	assert.Equal(t, want, in)
}