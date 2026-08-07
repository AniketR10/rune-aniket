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

package builtinfont

import (
	"bytes"
	"testing"

	gofont "github.com/go-text/typesetting/font"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/sfnt"
)

// TestEmojiTTFEmbeddedAndColor proves the bundled color emoji asset
// shipped, parses with the go-text font stack, and exposes 😀 as a PNG
// color bitmap glyph. This is the foundation the color-emoji renderer
// relies on; if the asset is missing or is not a bitmap-emoji font the
// whole color path is unavailable.
func TestEmojiTTFEmbeddedAndColor(t *testing.T) {
	require.NotEmpty(t, EmojiTTF, "NotoColorEmoji.ttf must be embedded")

	face, err := gofont.ParseTTF(bytes.NewReader(EmojiTTF))
	require.NoError(t, err, "bundled emoji font must parse with go-text")

	gid, ok := face.NominalGlyph('😀')
	require.True(t, ok, "grinning face must resolve to a glyph")

	bm, ok := face.GlyphData(gid).(gofont.GlyphBitmap)
	require.True(t, ok, "grinning face glyph must be a color bitmap")
	assert.Equal(t, gofont.PNG, bm.Format, "Noto color emoji stores PNG bitmaps")
	assert.Positive(t, bm.Width)
	assert.Positive(t, bm.Height)
	assert.NotEmpty(t, bm.Data)
}

// TestCJKTTCEmbedded proves the bundled Noto Sans CJK collection shipped
// and parses with x/image/font/sfnt, the stack the font Manager actually
// consumes, exposing the ten expected faces with Han coverage.
func TestCJKTTCEmbedded(t *testing.T) {
	require.NotEmpty(t, CJKTTC, "NotoSansCJK-Regular.ttc must be embedded")

	collection, err := sfnt.ParseCollection(CJKTTC)
	require.NoError(t, err, "bundled CJK collection must parse with sfnt")
	require.Equal(t, 10, collection.NumFonts(),
		"collection ships Sans and Mono for JP, KR, SC, TC and HK")

	f, err := collection.Font(7)
	require.NoError(t, err, "Noto Sans Mono CJK SC must be selectable")

	var buf sfnt.Buffer
	gid, err := f.GlyphIndex(&buf, '中')
	require.NoError(t, err)
	assert.NotZero(t, gid, "U+4E2D must resolve to a glyph")
}
