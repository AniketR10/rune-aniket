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

package apiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/unstablebuild/ox-api/auth"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"unstable.build/go-tui/cmd/rune/crashreport"
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
			// Crash reports are kept on local disk; uploading requires the
			// user to authenticate.
			_, _ = a.notifications.Notify(browserapi.LevelWarn,
				"Run the `login` command to upload pending crash reports.")
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
