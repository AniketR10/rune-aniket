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

package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfigStreamingOpenIsOff guards against accidentally
// flipping the streaming-open default on. Production opt-in (e.g.
// cmd/rune main) sets it explicitly via WithStreamingOpen so test
// fixtures and embedders remain unsurprised by the async semantics.
func TestDefaultConfigStreamingOpenIsOff(t *testing.T) {
	cfg := DefaultConfig()
	assert.False(t, cfg.StreamingOpen,
		"DefaultConfig must keep streaming open disabled")
}

// TestWithStreamingOpenSetsFlag confirms the option sets the flag.
func TestWithStreamingOpenSetsFlag(t *testing.T) {
	cfg := DefaultConfig()
	WithStreamingOpen(true)(&cfg)
	assert.True(t, cfg.StreamingOpen)
	WithStreamingOpen(false)(&cfg)
	assert.False(t, cfg.StreamingOpen)
}
