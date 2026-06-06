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
	"bytes"
	"io"
	"log/slog"
	"net/http"
)

// debugTransport logs the request and response bodies of every Gemini HTTP
// call when Config.DebugHTTP is enabled. It re-buffers both bodies so the
// underlying streaming reader is left intact for the genai SDK. This is the
// only way to inspect the genai wire (e.g. whether responses carry
// thoughtSignature), since the SDK exposes no body-level logging hook.
type debugTransport struct {
	base http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err == nil {
			slog.Debug("gemini http request",
				"method", req.Method, "url", req.URL.String(), "body", string(body))
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
	} else {
		slog.Debug("gemini http request", "method", req.Method, "url", req.URL.String())
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slog.Debug("gemini http error", "url", req.URL.String(), "error", err)
		return resp, err
	}

	// The completion endpoint streams SSE, so the response body must not be
	// drained eagerly. Tee it through a logger that records each chunk as it is
	// read by the genai SDK and flushes the accumulated body on close.
	if resp.Body != nil {
		resp.Body = &loggingBody{
			src:    resp.Body,
			status: resp.StatusCode,
			url:    req.URL.String(),
		}
	}
	return resp, nil
}

// loggingBody logs the streamed response body once it has been fully consumed.
type loggingBody struct {
	src    io.ReadCloser
	status int
	url    string
	buf    bytes.Buffer
	logged bool
}

func (b *loggingBody) Read(p []byte) (int, error) {
	n, err := b.src.Read(p)
	if n > 0 {
		b.buf.Write(p[:n])
	}
	if err == io.EOF {
		b.flush()
	}
	return n, err
}

func (b *loggingBody) Close() error {
	b.flush()
	return b.src.Close()
}

func (b *loggingBody) flush() {
	if b.logged {
		return
	}
	b.logged = true
	slog.Debug("gemini http response",
		"status", b.status, "url", b.url, "body", b.buf.String())
}
