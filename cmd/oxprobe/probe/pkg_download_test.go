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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/api/oxapi"
)

func TestPkgDownloadResult(t *testing.T) {
	t.Run("reads one byte of the signed url", func(t *testing.T) {
		var seen string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get("Range")
			w.Header().Set("Content-Range", "bytes 0-0/17")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("a"))
		}))
		defer srv.Close()

		res := PkgDownloadResult(context.Background(), srv.Client(), "darwin-arm64", srv.URL, true)

		require.Equal(t, "pkg_download", res.Layer)
		require.Equal(t, oxapi.CheckOK, res.Status, res.Detail)
		require.Equal(t, "bytes=0-0", seen)
		require.Equal(t, "signed download readable (darwin-arm64)", res.Detail)
	})

	// A backend that ignores Range must not drag the whole artifact back.
	t.Run("bounds the read when range is ignored", func(t *testing.T) {
		const size = 1 << 20
		var served atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chunk := make([]byte, 64<<10)
			for sent := 0; sent < size; sent += len(chunk) {
				n, err := w.Write(chunk)
				served.Add(int64(n))
				if err != nil {
					return
				}
				w.(http.Flusher).Flush()
			}
		}))

		res := PkgDownloadResult(context.Background(), srv.Client(), "darwin-arm64", srv.URL, true)
		srv.Close() // waits for the handler to notice the hang-up

		require.Equal(t, oxapi.CheckOK, res.Status, res.Detail)
		require.Less(t, served.Load(), int64(size/2))
	})

	t.Run("missing url fails", func(t *testing.T) {
		res := PkgDownloadResult(context.Background(), http.DefaultClient, "darwin-arm64", "", true)

		require.Equal(t, oxapi.CheckFail, res.Status)
		require.Contains(t, res.Detail, "no signed download url")
	})

	t.Run("non-200 fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		res := PkgDownloadResult(context.Background(), srv.Client(), "darwin-arm64", srv.URL, true)

		require.Equal(t, oxapi.CheckFail, res.Status)
		require.Contains(t, res.Detail, "403")
	})

	t.Run("zero bytes fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		res := PkgDownloadResult(context.Background(), srv.Client(), "darwin-arm64", srv.URL, true)

		require.Equal(t, oxapi.CheckFail, res.Status)
		require.Contains(t, res.Detail, "zero bytes")
	})
}
