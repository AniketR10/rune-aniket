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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/debug"
)

// TestFormatStampNormalizesToUTC pins that a non-UTC input is rendered
// with a trailing Z (UTC), so the emitted string never carries a zone
// offset that would depend on the builder's timezone.
func TestFormatStampNormalizesToUTC(t *testing.T) {
	loc := time.FixedZone("UTC+2", 2*60*60)
	got := formatStamp(time.Date(2026, 7, 9, 13, 35, 42, 0, loc))
	assert.Equal(t, "2026-07-09T11:35:42Z", got)
}

// TestFormatStampParsesWithGatingLayout is the contract test between
// this tool and the plan-gating parser: the emitted stamp must parse
// with debug.BuildDateLayout (the layout ide/ideplan.buildDate uses)
// and round-trip to the same instant. If either side changes layout,
// this trips.
func TestFormatStampParsesWithGatingLayout(t *testing.T) {
	want := time.Date(2026, 7, 9, 13, 35, 42, 0, time.UTC)
	stamp := formatStamp(want)

	parsed, err := time.Parse(debug.BuildDateLayout, stamp)
	require.NoError(t, err, "emitted stamp must parse with the gating layout")
	assert.True(t, parsed.UTC().Equal(want),
		"emitted stamp must round-trip to the original instant")
}
