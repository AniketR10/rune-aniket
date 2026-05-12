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

package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAnimConfig(t *testing.T, src string) ideConfig {
	t.Helper()
	cfg, err := decodeStarlark(src, nil, nil)
	require.NoError(t, err)
	return ideConfig{
		cfg:    cfg,
		errors: make(map[string]error),
	}
}

// TestAnimationsDefaultsEnabled asserts that both animation toggles
// default to enabled when the rune.star config does not mention them.
func TestAnimationsDefaultsEnabled(t *testing.T) {
	c := newAnimConfig(t, `config = {}`)
	assert.True(t, c.animationsLoadingWorkspace())
	assert.True(t, c.animationsOpenWorkspace())
	assert.Empty(t, c.errors)
}

// TestAnimationsEmptySection keeps the defaults.
func TestAnimationsEmptySection(t *testing.T) {
	c := newAnimConfig(t, `config = {"animations": {}}`)
	assert.True(t, c.animationsLoadingWorkspace())
	assert.True(t, c.animationsOpenWorkspace())
	assert.Empty(t, c.errors)
}

// TestAnimationsExplicitFalseDisables verifies that explicitly
// setting either toggle to False reports disabled.
func TestAnimationsExplicitFalseDisables(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "animations": {
        "loading_workspace": False,
        "open_workspace":    False,
    },
}
`)
	assert.False(t, c.animationsLoadingWorkspace())
	assert.False(t, c.animationsOpenWorkspace())
	assert.Empty(t, c.errors)
}

// TestAnimationsExplicitTrue is a no-op compared to the defaults but
// pins behavior in case the default ever changes.
func TestAnimationsExplicitTrue(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "animations": {
        "loading_workspace": True,
        "open_workspace":    True,
    },
}
`)
	assert.True(t, c.animationsLoadingWorkspace())
	assert.True(t, c.animationsOpenWorkspace())
	assert.Empty(t, c.errors)
}

// TestAnimationsLoadingIndependentOfOpen verifies that the two
// toggles are independent: disabling one does not affect the other.
func TestAnimationsLoadingIndependentOfOpen(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "animations": {
        "loading_workspace": False,
    },
}
`)
	assert.False(t, c.animationsLoadingWorkspace())
	assert.True(t, c.animationsOpenWorkspace())
}

// TestAnimationsWrongType records an error and falls back to enabled
// so a typo never silently disables the animation.
func TestAnimationsWrongType(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "animations": {
        "loading_workspace": "yes",
    },
}
`)
	assert.True(t, c.animationsLoadingWorkspace(),
		"wrong type must fall back to enabled")
	require.Contains(t, c.errors, "animations.loading_workspace")
}

// TestAnimationsSectionWrongType records an error and falls back to
// enabled when the whole animations key is not a dict.
func TestAnimationsSectionWrongType(t *testing.T) {
	c := newAnimConfig(t, `config = {"animations": "off"}`)
	assert.True(t, c.animationsLoadingWorkspace())
	assert.True(t, c.animationsOpenWorkspace())
	require.Contains(t, c.errors, "animations")
}
