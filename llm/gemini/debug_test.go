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

package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// TestDebugTransportPreservesBody verifies the debug round-tripper logs but
// leaves the request/response bodies intact for the caller.
func TestDebugTransportPreservesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"in":1}`, string(body))
		_, _ = w.Write([]byte(`{"out":2}`))
	}))
	t.Cleanup(srv.Close)

	rt := &debugTransport{base: http.DefaultTransport}
	req, err := http.NewRequest(http.MethodPost, srv.URL,
		io.NopCloser(stringReader(`{"in":1}`)))
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, `{"out":2}`, string(got))
}

// TestDebugHTTPStreamsEndToEnd verifies enabling DebugHTTP does not break the
// streaming completion path (the response body must not be drained eagerly).
func TestDebugHTTPStreamsEndToEnd(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`,
	}
	srv, cfg := sseServer(t, chunks)
	_ = srv
	cfg.DebugHTTP = true
	svc := NewClient("k", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
	})

	var text strings.Builder
	for _, ev := range events {
		if ev.Type == llmapi.EventTextDelta {
			text.WriteString(ev.Text)
		}
	}
	assert.Equal(t, "hi", text.String())
}

type stringReader string

func (s stringReader) Read(p []byte) (int, error) {
	n := copy(p, s)
	if n < len(s) {
		return n, nil
	}
	return n, io.EOF
}
