// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
