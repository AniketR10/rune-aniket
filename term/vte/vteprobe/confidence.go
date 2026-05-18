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

package vteprobe

import "github.com/unstablebuild/rune-go-sdk/term"

// computeConfidence combines several lightweight signals into a value
// in [0, 1]:
//   - alignment coverage: fraction of rows that matched their file line;
//   - whether the cursor row itself has positive alignment;
//   - whether the cursor cell looks like real content (non-empty rune).
//
// The function is intentionally simple — callers threshold the result
// against the minConfidence value passed to New, so subtle tuning is
// the caller's concern.
func computeConfidence(
	a alignment,
	w wrapInfo,
	rows []extractedRow,
	cur term.Coordinates,
) float64 {
	conf := a.coverage
	if conf > 1 {
		conf = 1
	}
	if cur.Y >= 0 && cur.Y < len(w.rowFileLine) && w.rowFileLine[cur.Y] > 0 {
		conf += 0.1
	}
	if cur.Y >= 0 && cur.Y < len(rows) {
		row := rows[cur.Y]
		idx := row.runeColAt(cur.X)
		if idx >= 0 && idx < len(row.runes) && row.runes[idx] != ' ' && row.runes[idx] != 0 {
			conf += 0.05
		}
	}
	if conf > 1 {
		conf = 1
	}
	return conf
}
