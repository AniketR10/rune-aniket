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
