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

package apiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"unstable.build/rune/auth"
	"unstable.build/rune/cmd/rune/crashreport"
)

const reportUploadTimeout = 30 * time.Second
const reportContentType = "application/yaml"

// PostReport uploads a report payload to the server.
// It uses the OAuth token source for authenticated requests.
func (a *Client) PostReport(ctx context.Context, payload crashreport.Payload) error {
	if len(payload.YAML) == 0 {
		return fmt.Errorf("empty report payload")
	}

	u := *a.httpEndpointURL
	u.Path = "/api/reports"
	url := u.String()

	ctx, cancel := context.WithTimeout(ctx, reportUploadTimeout)
	defer cancel()

	r, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload.YAML))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	r.Header.Set("Content-Type", reportContentType)

	// Use the OAuth token source for authenticated requests.
	token, err := a.tokenSource.Token()
	if err != nil {
		if errors.Is(err, auth.ErrNotAuthenticated) {
			return auth.ErrNotAuthenticated
		}
		return fmt.Errorf("get auth token: %w", err)
	}
	if token.Valid() {
		r.Header.Set("Authorization",
			fmt.Sprintf("%s %s", token.TokenType, token.AccessToken))
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return fmt.Errorf("post report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("report upload: status %d", resp.StatusCode)
	}
	return nil
}
