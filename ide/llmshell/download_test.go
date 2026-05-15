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


package llmshell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHumanBytes verifies the size scaling helper used in download
// summaries.
func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	} {
		got := humanBytes(tc.n)
		assert.Equal(t, tc.want, got)
	}
}

// TestScaleProgressBytes selects a unit that keeps the bar's
// denominator above minTicks (~100) so the progress bar has at least
// ~1% granularity.
func TestScaleProgressBytes(t *testing.T) {
	const minTicks = int64(100)
	cases := []struct {
		downloaded int64
		total      int64
	}{
		{0, 1024},                  // sub-100 -> B
		{500, 200 * 1024},          // KiB
		{1024 * 1024, 250 * 1024 * 1024}, // MiB
		{1024 * 1024, 250 * 1024 * 1024 * 1024}, // GiB
	}
	for _, tc := range cases {
		_, scaledTotal, unit := scaleProgressBytes(tc.downloaded, tc.total)
		switch unit {
		case "B":
			assert.Less(t, tc.total/1024, minTicks)
		default:
			assert.GreaterOrEqual(t, scaledTotal, minTicks)
		}
	}
}

// TestScaleProgressBytesClampsOverflow keeps the downloaded portion
// from exceeding total when a buggy source reports a larger count.
func TestScaleProgressBytesClampsOverflow(t *testing.T) {
	d, total, _ := scaleProgressBytes(2000, 1000)
	assert.LessOrEqual(t, d, total)
}

// TestDownloadUsageIsValidMarkdown verifies the help block is not empty
// and references the new command path.
func TestDownloadUsageIsValidMarkdown(t *testing.T) {
	assert.NotEmpty(t, downloadUsage)
	assert.True(t, strings.Contains(downloadUsage, "models local download"))
}
