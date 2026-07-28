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

package utf8validate

import (
	"bytes"
	"crypto/sha256"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"ascii", "hello", "hello"},
		{"valid multibyte", "héllo · 世界", "héllo · 世界"},
		{"isolated continuation", "ok\x80bad", "ok\ufffdbad"},
		{"truncated multibyte prefix", "ok\xc3", "ok\ufffd"},
		{"lone 0xff", "ok\xffbad", "ok\ufffdbad"},
		{"multiple invalid", "\xff\xfe\xfdtrailing", "\ufffd\ufffd\ufffdtrailing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sanitize(tc.in)
			assert.Equal(t, tc.want, got)
			assert.True(t, utf8.ValidString(got))
		})
	}
}

func TestSanitize_ValidReturnsSameUnderlyingBytes(t *testing.T) {
	in := "héllo · 世界"
	out := Sanitize(in)
	require.Equal(t, in, out)
	// Validity fast path returns the exact same string header (no copy).
	assert.Equal(t, unsafe.StringData(in), unsafe.StringData(out))
}

func TestCountInvalidBytes(t *testing.T) {
	cases := []string{
		"",
		"hello",
		"héllo · 世界",
		"ok\x80bad",
		"ok\xc3",
		"ok\xffbad",
		"\xff\xfe\xfdtrailing",
	}
	for _, in := range cases {
		got := CountInvalidBytes(in)
		// Count equals number of \ufffd runes inserted by Sanitize;
		// none of our inputs contain a pre-existing U+FFFD.
		sanitized := Sanitize(in)
		assert.Equal(t, strings.Count(sanitized, "\ufffd"), got, "input=%q", in)
	}
}

func TestIsBinary(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", nil, false},
		{"pure ascii", []byte("hello world\n"), false},
		{"utf8 multibyte", []byte("héllo · 世界\n"), false},
		{"nul in first 8k", append([]byte("text "), 0x00, 'm', 'o', 'r', 'e'), true},
		{"latin1 single bad byte", []byte("normal log line with one \xff oddity\n"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsBinary(tc.data))
		})
	}

	t.Run("high entropy random bytes", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))
		data := make([]byte, 1024)
		for i := range data {
			data[i] = byte(0x80 + rng.Intn(0x80)) // upper half only
		}
		assert.True(t, IsBinary(data))
	})

	t.Run("nul beyond scan limit is ignored", func(t *testing.T) {
		data := bytes.Repeat([]byte("a"), binaryScanLimit+1)
		data[binaryScanLimit] = 0x00
		assert.False(t, IsBinary(data))
	})
}

func TestBinaryStub(t *testing.T) {
	sum := sha256.Sum256([]byte("hello"))
	got := BinaryStub("a.out", 1234, sum)
	// Header carries identity (name, size, sha256).
	header := regexp.MustCompile(`^<binary file: a\.out, 1234 bytes, sha256=[0-9a-f]{64}`)
	assert.Regexp(t, header, got)
	// Body steers the model toward bash-based inspection rather than
	// retrying read_file on the same path.
	assert.Contains(t, got, "use bash")
	assert.Contains(t, got, "xxd")
}

func TestInvalidBytesMarker(t *testing.T) {
	assert.Equal(t, "", InvalidBytesMarker(0))
	assert.Equal(t, "", InvalidBytesMarker(-1))
	assert.Equal(t, "(1 invalid UTF-8 byte replaced with U+FFFD)", InvalidBytesMarker(1))
	assert.Equal(t, "(7 invalid UTF-8 bytes replaced with U+FFFD)", InvalidBytesMarker(7))
}
