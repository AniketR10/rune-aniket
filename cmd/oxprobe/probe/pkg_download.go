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

package probe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/unstablebuild/ox-api/api/oxapi"
)

// PkgDownloadResult verifies the anonymous artifact-download leg of a
// real install: it fetches the signed download URL that ox-api minted in
// the verbose /health report and confirms bytes come back. ox-api owns
// signing (which the server-side release_signing checker covers); this
// proves the signed URL is actually usable by an unauthenticated client,
// exactly as `pkg install ada` relies on.
//
// signedURL is the per-arch URL from Report.SignedDownloads. arch is only
// used for the result detail. The returned result always uses the
// "pkg_download" layer so it slots beside the other probe results.
func PkgDownloadResult(ctx context.Context, client *http.Client, arch, signedURL string, critical bool) oxapi.CheckResult {
	return run(ctx, "pkg_download", critical, func(ctx context.Context) (string, error) {
		if signedURL == "" {
			return "", fmt.Errorf("no signed download url for %s in /health report", arch)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
		if err != nil {
			return "", err
		}
		// The URL is signed for GET, so HEAD is not an option; a ranged
		// GET proves it is usable without paying egress for the whole
		// artifact every minute.
		req.Header.Set("Range", "bytes=0-0")
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("download %s: %w", arch, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			return "", fmt.Errorf("%s signed download status %d", arch, resp.StatusCode)
		}
		// Bounded regardless of whether the backend honoured Range.
		n, err := io.CopyN(io.Discard, resp.Body, 1)
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read %s body: %w", arch, err)
		}
		if n == 0 {
			return "", fmt.Errorf("%s signed download returned zero bytes", arch)
		}
		return fmt.Sprintf("signed download readable (%s)", arch), nil
	})
}
