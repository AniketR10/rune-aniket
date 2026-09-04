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

package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// runHTTP dispatches a single http-type hook. It POSTs the JSON
// envelope to the configured URL and parses the response body as a
// HookOutput. Non-2xx responses and timeouts produce non-blocking
// warnings; the runner continues.
func (r *Runner) runHTTP(
	ctx context.Context, h Hook, payloadJSON []byte,
) hookResult {
	cctx, cancel := context.WithTimeout(ctx, h.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodPost, h.URL, bytes.NewReader(payloadJSON))
	if err != nil {
		return hookResult{warn: fmt.Errorf("http hook: build request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range h.Headers {
		req.Header.Set(k, interpolateEnv(v, h.AllowedEnvVars))
	}

	client := r.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return hookResult{warn: fmt.Errorf("http hook: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOutputBytes))
	if err != nil {
		return hookResult{warn: fmt.Errorf("http hook: read body: %w", err)}
	}
	res := hookResult{stdout: string(body)}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.warn = fmt.Errorf("http hook: status %d", resp.StatusCode)
		return res
	}
	if out, ok := parseHookOutput(body); ok {
		res.output = out
	}
	return res
}

// interpolateEnv replaces ${VAR} occurrences in s when VAR is in
// allowed. Variables not on the allow-list are left literal.
func interpolateEnv(s string, allowed []string) string {
	if len(allowed) == 0 || !strings.Contains(s, "${") {
		return s
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, v := range allowed {
		allowedSet[v] = struct{}{}
	}
	return os.Expand(s, func(name string) string {
		if _, ok := allowedSet[name]; !ok {
			return "${" + name + "}"
		}
		return os.Getenv(name)
	})
}
