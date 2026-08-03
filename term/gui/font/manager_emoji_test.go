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

package font

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"

	"unstable.build/go-tui/term/gui/font/builtinfont"
)

// stubFindFont resolves family→path mappings so the emoji resolver runs
// deterministically regardless of the host's installed fonts.
type stubFindFont struct {
	byFamily map[string]string
}

func (s stubFindFont) findByFamily(family string) (iterator.Iterator[metadata], error) {
	path, ok := s.byFamily[family]
	if !ok {
		return nil, fmt.Errorf("font '%s' not found", family)
	}
	return iterator.FromSlice([]metadata{{family: family, path: path}}), nil
}

func (s stubFindFont) list() (iterator.Iterator[metadata], error) {
	return iterator.Empty[metadata](), nil
}

// TestEmojiFacePrefersSystemFont asserts a usable system font is
// preferred over the bundled fallback and labeled as such.
func TestEmojiFacePrefersSystemFont(t *testing.T) {
	// Advertise the bundled font under every candidate family so the
	// first one the resolver tries is usable on any platform.
	path := filepath.Join(t.TempDir(), "SystemColorEmoji.ttf")
	require.NoError(t, os.WriteFile(path, builtinfont.EmojiTTF, 0o600))

	families := emojiFontFamilies()
	if len(families) == 0 {
		t.Skip("platform advertises no system emoji families")
	}
	byFamily := make(map[string]string, len(families))
	for _, f := range families {
		byFamily[f] = path
	}

	m := &Manager{findfont: stubFindFont{byFamily: byFamily}}
	face, source := m.EmojiFace()

	require.NotNil(t, face)
	assert.Equal(t, "system: "+families[0], source)
	assert.True(t, face.Has([]rune{'😀'}))
}

// TestEmojiFaceFallsBackToBundled asserts that with no usable system
// font the resolver falls back to the bundled Noto, holding even on CI
// hosts with no color-emoji fonts.
func TestEmojiFaceFallsBackToBundled(t *testing.T) {
	m := &Manager{findfont: stubFindFont{byFamily: map[string]string{}}}
	face, source := m.EmojiFace()

	require.NotNil(t, face)
	assert.Equal(t, "bundled: Noto Color Emoji", source)
	assert.True(t, face.Has([]rune{'😀'}))
}

// TestEmojiFaceSkipsUnusableSystemFont asserts a family resolving to a
// non-color font is skipped in favor of the bundled fallback.
func TestEmojiFaceSkipsUnusableSystemFont(t *testing.T) {
	families := emojiFontFamilies()
	if len(families) == 0 {
		t.Skip("platform advertises no system emoji families")
	}
	// A non-font file makes NewFaceFromFile reject the candidate.
	bad := filepath.Join(t.TempDir(), "notafont.ttf")
	require.NoError(t, os.WriteFile(bad, []byte("not a font"), 0o600))

	m := &Manager{findfont: stubFindFont{byFamily: map[string]string{families[0]: bad}}}
	face, source := m.EmojiFace()

	require.NotNil(t, face)
	assert.Equal(t, "bundled: Noto Color Emoji", source)
}

// TestEmojiFaceResolvesOnce asserts resolution happens once and is
// cached, so repeated calls do not re-parse a system font collection.
func TestEmojiFaceResolvesOnce(t *testing.T) {
	m := &Manager{findfont: stubFindFont{byFamily: map[string]string{}}}
	face1, source1 := m.EmojiFace()
	face2, source2 := m.EmojiFace()

	assert.Same(t, face1, face2, "face is cached across calls")
	assert.Equal(t, source1, source2)
}
