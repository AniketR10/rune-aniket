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
