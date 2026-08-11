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

package gui

import "testing"

func TestTPSForRefreshRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		hz       int
		expected int
	}{
		{name: "unknown falls back", hz: 0, expected: fallbackTPS},
		{name: "negative falls back", hz: -1, expected: fallbackTPS},
		{name: "standard 60Hz", hz: 60, expected: 60},
		{name: "promotion 120Hz", hz: 120, expected: 120},
		{name: "gaming 144Hz", hz: 144, expected: 144},
		{name: "slow mode clamps up", hz: 30, expected: minTPS},
		{name: "extreme clamps down", hz: 500, expected: maxTPS},
		{name: "min boundary", hz: minTPS, expected: minTPS},
		{name: "max boundary", hz: maxTPS, expected: maxTPS},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tpsForRefreshRate(tc.hz); got != tc.expected {
				t.Fatalf("tpsForRefreshRate(%d) = %d, expected %d",
					tc.hz, got, tc.expected)
			}
		})
	}
}

func TestApplyTPSNilMonitorFallsBack(t *testing.T) {
	g := &GUI{}
	g.applyTPS(nil)
	if g.currentTPS != fallbackTPS {
		t.Fatalf("currentTPS = %d, expected fallback %d",
			g.currentTPS, fallbackTPS)
	}
}
