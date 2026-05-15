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

package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

// recordingHandler is a Handler stub that captures SetLocationList
// arguments so tests can assert dispatch fired.
type recordingHandler struct {
	nopPubHandler
	calls int
}

func (r *recordingHandler) SetLocationList(
	_ textapi.LocationPriority, _ string, _ LocationList,
) {
	r.calls++
}

// TestSyntaxLocationSetterIsNopBeforePublish reproduces the byoe
// crash scenario: the syntax tree's async parse fires
// SetLocationList BEFORE newFileBuffer publishes the handler (e.g.
// because Edit returned an error and newFileBuffer returned early).
// The setter must not dereference a nil handler.
func TestSyntaxLocationSetterIsNopBeforePublish(t *testing.T) {
	setter, _ := newSyntaxLocationSetter()
	assert.NotPanics(t, func() {
		setter.SetLocationList(nil)
	})
}

// TestSyntaxLocationSetterDispatchesAfterPublish verifies the
// setter resumes normal dispatch once the handler is published.
func TestSyntaxLocationSetterDispatchesAfterPublish(t *testing.T) {
	setter, publish := newSyntaxLocationSetter()
	rec := &recordingHandler{}
	publish(rec)
	setter.SetLocationList(nil)
	assert.Equal(t, 1, rec.calls)
}

// TestSyntaxLocationSetterPublishNilDoesNotPanic guards against a
// caller publishing nil after a failed Edit. Even in that case the
// setter must remain a no-op rather than crash.
func TestSyntaxLocationSetterPublishNilDoesNotPanic(t *testing.T) {
	setter, publish := newSyntaxLocationSetter()
	publish(nil)
	assert.NotPanics(t, func() {
		setter.SetLocationList(nil)
	})
}
