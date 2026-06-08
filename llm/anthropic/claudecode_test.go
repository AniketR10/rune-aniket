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

package anthropic

import (
	"bytes"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestXXHash64MatchesReferenceSeedZero cross-checks the inline xxHash64 against
// the in-tree cespare/xxhash (seed 0) so the algorithm is provably correct.
func TestXXHash64MatchesReferenceSeedZero(t *testing.T) {
	inputs := [][]byte{
		nil,
		[]byte("a"),
		[]byte("abc"),
		[]byte("the quick brown fox jumps over the lazy dog"),
		bytes.Repeat([]byte("x"), 31),
		bytes.Repeat([]byte("y"), 32),
		bytes.Repeat([]byte("z"), 33),
		bytes.Repeat([]byte("claude-code-billing"), 100),
	}
	for _, in := range inputs {
		assert.Equalf(t, xxhash.Sum64(in), xxHash64Checksum(in, 0),
			"seed-0 mismatch for len=%d", len(in))
	}
}

func TestComputeFingerprintDeterministic(t *testing.T) {
	const text = "You are a helpful assistant for the rune editor project."
	fp1 := computeFingerprint(text, claudeCodeVersion)
	fp2 := computeFingerprint(text, claudeCodeVersion)
	require.Equal(t, fp1, fp2)
	assert.Len(t, fp1, 3)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{3}$`), fp1)

	// Short text falls back to '0' for out-of-range indices but is still stable.
	fpShort := computeFingerprint("abc", claudeCodeVersion)
	assert.Equal(t, fpShort, computeFingerprint("abc", claudeCodeVersion))
}

func TestSignClaudeCodeBodyProducesStable5HexCCH(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63.abc; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`)

	signed := signClaudeCodeBody(body)
	header := gjson.GetBytes(signed, "system.0.text").String()

	cch := regexp.MustCompile(`\bcch=([0-9a-f]{5});`).FindStringSubmatch(header)
	require.Len(t, cch, 2, "expected a 5-hex cch token, got %q", header)
	assert.NotEqual(t, "00000", cch[1])

	// Signing is deterministic for the same body.
	again := signClaudeCodeBody(body)
	assert.Equal(t, string(signed), string(again))
}

func TestSignClaudeCodeBodyNoopWithoutBillingHeader(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"You are a helpful assistant."}],"messages":[]}`)
	assert.Equal(t, string(body), string(signClaudeCodeBody(body)))
}

func TestSignClaudeCodeBodyIsIndependentOfInitialCCH(t *testing.T) {
	withZero := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63.abc; cc_entrypoint=cli; cch=00000;"}]}`)
	withOther := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63.abc; cc_entrypoint=cli; cch=12345;"}]}`)
	assert.Equal(t, string(signClaudeCodeBody(withZero)), string(signClaudeCodeBody(withOther)),
		"cch must be derived from the body with cch zeroed, not from the incoming cch")
}

func TestClaudeCodeMiddlewareSignsBody(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.63.abc; cc_entrypoint=cli; cch=00000;"}],"messages":[]}`)
	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/messages",
		io.NopCloser(bytes.NewReader(body)))
	require.NoError(t, err)

	var seen []byte
	mw := claudeCodeMiddleware()
	_, _ = mw(req, func(r *http.Request) (*http.Response, error) {
		seen, _ = io.ReadAll(r.Body)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})

	header := gjson.GetBytes(seen, "system.0.text").String()
	assert.Regexp(t, regexp.MustCompile(`\bcch=[0-9a-f]{5};`), header)
	assert.NotContains(t, header, "cch=00000;")
	assert.Equal(t, int64(len(seen)), req.ContentLength)
}
