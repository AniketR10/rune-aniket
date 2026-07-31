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
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/ox-api/api/oxapi"

	"unstable.build/go-tui/ide/idepkg"
)

// TestReleasesURLMatchesEditor pins the probe's base URL to the shape the
// editor installs through. The probe cannot import idepkg outside tests
// without pulling the whole editor into the probe binary, so this is what
// stops the two from drifting.
func TestReleasesURLMatchesEditor(t *testing.T) {
	for _, base := range []string{"https://api.rune.build", "http://127.0.0.1:8080"} {
		require.Equal(t,
			idepkg.ReleasesURL(base, "darwin-arm64"),
			releasesURL(mustURL(t, base), "darwin-arm64"))
	}
}

// TestOwnedLanguagesMatchesScript keeps the probe's ownership rule tied to
// deploy/package_versions.sh, which is where the set is defined.
func TestOwnedLanguagesMatchesScript(t *testing.T) {
	script, err := os.ReadFile("../../../deploy/package_versions.sh")
	require.NoError(t, err)

	var want []string
	_, rest, ok := strings.Cut(string(script), "OWNED_LANGUAGES='")
	require.True(t, ok, "OWNED_LANGUAGES not found in package_versions.sh")
	literal, _, ok := strings.Cut(rest, "'")
	require.True(t, ok)
	require.NoError(t, json.Unmarshal([]byte(literal), &want))

	require.ElementsMatch(t, want, ownedLanguages)
}

func TestPkgLatestProbe(t *testing.T) {
	tests := []struct {
		name  string
		index releaseIndex
		archs []string
		want  oxapi.CheckStatus
		// checked is the arch/package pairs whose artifact must be
		// fetched. nil skips the assertion.
		checked []string
		detail  []string
	}{
		{
			name: "checks the packages we own and skips grammars",
			index: releaseIndex{
				"darwin-arm64": {
					"rune-agent": published("v1.1.2"),
					"rg":         published("v1.0.0"),
					"go":         language(published("v1.26.5")),
					"ada":        language(published("v0.0.1")),
					"zig":        language(published("v0.0.1")),
				},
			},
			archs:   []string{"darwin-arm64"},
			want:    oxapi.CheckOK,
			checked: []string{"darwin-arm64/go", "darwin-arm64/rg", "darwin-arm64/rune-agent"},
			detail:  []string{"3 latest versions downloadable (3 packages, 1 archs)"},
		},
		{
			// Only darwin-arm64 marks grammars, so an arch-local rule
			// would drag every grammar on the other archs into the owned
			// set. The marker has to be unioned across archs.
			name: "grammar marker on one arch filters every arch",
			index: releaseIndex{
				"darwin-arm64": {
					"ada":        language(published("v0.0.1")),
					"rune-agent": published("v1.1.2"),
				},
				"linux-amd64": {
					"ada":        published("v0.0.1"),
					"rune-agent": published("v1.1.2"),
				},
			},
			archs:   []string{"darwin-arm64", "linux-amd64"},
			want:    oxapi.CheckOK,
			checked: []string{"darwin-arm64/rune-agent", "linux-amd64/rune-agent"},
		},
		{
			// The rune-agent v1.1.2 regression: the bundle row exists and
			// the API mints a signed URL, but the object is not in the
			// bucket, so every `pkg install rune-agent` 404s.
			name: "latest bundle exists but artifact is missing",
			index: releaseIndex{
				"darwin-arm64": {"rune-agent": {latest: "v1.1.2", bundles: []string{"v1.1.2"}}},
			},
			archs: []string{"darwin-arm64"},
			want:  oxapi.CheckFail,
			detail: []string{
				"1/1 latest versions not downloadable",
				"darwin-arm64/rune-agent: v1.1.2: artifact: status 404",
			},
		},
		{
			name: "latest points at a version with no bundle",
			index: releaseIndex{
				"darwin-arm64": {"rune-agent": {latest: "v9.9.9", blobs: []string{"v1.1.0"}}},
			},
			archs:  []string{"darwin-arm64"},
			want:   oxapi.CheckFail,
			detail: []string{"darwin-arm64/rune-agent: v9.9.9: bundle: status 404"},
		},
		{
			// package_versions.sh skips these rather than reporting them:
			// not publishing a package for an arch is routine.
			name: "package without a latest is skipped, not failed",
			index: releaseIndex{
				"darwin-arm64": {
					"rune-agent": published("v1.1.2"),
					"runectl":    {},
				},
			},
			archs:   []string{"darwin-arm64"},
			want:    oxapi.CheckOK,
			checked: []string{"darwin-arm64/rune-agent"},
			detail:  []string{"1 latest versions downloadable (1 packages, 1 archs)"},
		},
		{
			name:   "registry advertising nothing installable fails",
			index:  releaseIndex{"darwin-arm64": {"ada": language(published("v0.0.1"))}},
			archs:  []string{"darwin-arm64"},
			want:   oxapi.CheckFail,
			detail: []string{"no owned package advertises a Latest version"},
		},
		{
			name:   "unreachable registry fails",
			index:  releaseIndex{},
			archs:  []string{"darwin-arm64"},
			want:   oxapi.CheckFail,
			detail: []string{"list darwin-arm64: status 404"},
		},
		{
			name: "artifact serves zero bytes",
			index: releaseIndex{
				"darwin-arm64": {
					"rune-agent": {latest: "v1.1.2", blobs: []string{"v1.1.2"}, empty: true},
				},
			},
			archs:  []string{"darwin-arm64"},
			want:   oxapi.CheckFail,
			detail: []string{"darwin-arm64/rune-agent: v1.1.2: artifact is empty"},
		},
		{
			name: "every broken target is reported, not just the first",
			index: releaseIndex{
				"darwin-amd64": {
					"fuzzy-search": published("v1.1.2"),
					"rune-agent":   {latest: "v1.1.2", bundles: []string{"v1.1.2"}},
				},
				"darwin-arm64": {
					"fuzzy-search": published("v1.1.2"),
					"rune-agent":   {latest: "v1.1.2", bundles: []string{"v1.1.2"}},
				},
			},
			archs: []string{"darwin-amd64", "darwin-arm64"},
			want:  oxapi.CheckFail,
			detail: []string{
				"2/4 latest versions not downloadable",
				"darwin-amd64/rune-agent: v1.1.2: artifact: status 404",
				"darwin-arm64/rune-agent: v1.1.2: artifact: status 404",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, downloads := newReleaseServer(tt.index)
			defer srv.Close()

			p := PkgLatestProbe{
				APIURL:     mustURL(t, srv.URL),
				Archs:      tt.archs,
				IsCritical: true,
				Client:     srv.Client(),
			}

			res := p.Run(context.Background())

			require.Equal(t, "pkg_latest", res.Layer)
			require.True(t, res.Critical)
			require.Equal(t, tt.want, res.Status, res.Detail)
			for _, want := range tt.detail {
				require.Contains(t, res.Detail, want)
			}
			if tt.checked != nil {
				require.Equal(t, tt.checked, downloads.seen())
			}
		})
	}
}

// TestPkgLatestProbeDoesNotDrainArtifacts guards the bandwidth budget: the
// probe runs every minute against artifacts that run to hundreds of
// megabytes, so it must ask the bucket for a single byte rather than
// download the bundle the way a real install does. Aborting an unranged
// GET is not enough — those bytes are already in flight, and billed.
func TestPkgLatestProbeDoesNotDrainArtifacts(t *testing.T) {
	const size = 8 << 20
	state := &pkgState{latest: "v1.1.2", blobs: []string{"v1.1.2"}, size: size}
	srv, _ := newReleaseServer(releaseIndex{"darwin-arm64": {"rune-agent": state}})

	p := PkgLatestProbe{
		APIURL: mustURL(t, srv.URL),
		Archs:  []string{"darwin-arm64"},
		Client: srv.Client(),
	}

	require.Equal(t, oxapi.CheckOK, p.Run(context.Background()).Status)
	srv.Close()

	require.Equal(t, "bytes=0-0", state.rangeSeen())
	require.LessOrEqual(t, state.bytesServed(), int64(1))
}

// TestPkgLatestProbeBoundsReadWhenRangeIgnored keeps the bandwidth budget
// independent of bucket configuration: a backend that answers 200 with the
// whole object must still not be drained.
func TestPkgLatestProbeBoundsReadWhenRangeIgnored(t *testing.T) {
	const size = 8 << 20
	state := &pkgState{
		latest: "v1.1.2", blobs: []string{"v1.1.2"}, size: size, ignoreRange: true,
	}
	srv, _ := newReleaseServer(releaseIndex{"darwin-arm64": {"rune-agent": state}})

	p := PkgLatestProbe{
		APIURL: mustURL(t, srv.URL),
		Archs:  []string{"darwin-arm64"},
		Client: srv.Client(),
	}

	require.Equal(t, oxapi.CheckOK, p.Run(context.Background()).Status)
	srv.Close() // waits for the artifact handler to notice the hang-up

	require.Less(t, state.bytesServed(), int64(size/2))
}

// TestPkgLatestProbeKeepsSignedURLsOutOfPages guards the credential in the
// signed URL: net/http renders the full request URL in url.Error, and that
// error text lands in a PagerDuty payload.
func TestPkgLatestProbeKeepsSignedURLsOutOfPages(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close() // nothing is listening, so the artifact leg fails to dial

	const secret = "x-goog-signature=deadbeef"
	state := &pkgState{
		latest: "v1.1.2", blobs: []string{"v1.1.2"},
		signedURL: dead.URL + "/blob?" + secret,
	}
	srv, _ := newReleaseServer(releaseIndex{"darwin-arm64": {"rune-agent": state}})
	defer srv.Close()

	p := PkgLatestProbe{
		APIURL: mustURL(t, srv.URL),
		Archs:  []string{"darwin-arm64"},
		Client: srv.Client(),
	}

	res := p.Run(context.Background())

	require.Equal(t, oxapi.CheckFail, res.Status)
	require.Contains(t, res.Detail, "v1.1.2: artifact: ")
	require.NotContains(t, res.Detail, secret)
}

type pkgState struct {
	latest   string
	language bool
	// bundles are versions the release index knows about. blobs are
	// versions whose artifact is also in the bucket; listing a version
	// there implies a bundle row for it too.
	bundles []string
	blobs   []string
	empty   bool
	// size, when set, makes the artifact handler stream that many bytes
	// so a test can observe how much the probe actually pulls.
	size int
	// ignoreRange models a backend that answers a ranged GET with the
	// whole object.
	ignoreRange bool
	// signedURL, when set, replaces the bundle leg's minted URL.
	signedURL string

	mu     sync.Mutex
	served int64
	rngHdr string
}

func published(version string) *pkgState {
	return &pkgState{latest: version, blobs: []string{version}}
}

func language(s *pkgState) *pkgState {
	s.language = true
	return s
}

func (s *pkgState) hasBundle(version string) bool {
	return slices.Contains(s.bundles, version) || slices.Contains(s.blobs, version)
}

// serve streams total bytes in chunks, recording how many the client
// actually took before hanging up.
func (s *pkgState) serve(w http.ResponseWriter, total int) {
	chunk := make([]byte, min(64<<10, total))
	flusher, _ := w.(http.Flusher)
	for sent := 0; sent < total; sent += len(chunk) {
		n, err := w.Write(chunk[:min(len(chunk), total-sent)])
		s.mu.Lock()
		s.served += int64(n)
		s.mu.Unlock()
		if err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func (s *pkgState) bytesServed() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.served
}

func (s *pkgState) recordRange(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rngHdr = v
}

func (s *pkgState) rangeSeen() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rngHdr
}

type releaseIndex map[string]map[string]*pkgState

type downloadLog struct {
	mu    sync.Mutex
	seen_ []string
}

func (l *downloadLog) record(target string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen_ = append(l.seen_, target)
}

func (l *downloadLog) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := append([]string(nil), l.seen_...)
	slices.Sort(out)
	return out
}

// newReleaseServer serves the subset of the cdnrelease HTTP surface the
// probe walks, plus the bucket the signed URLs point at, so a missing
// artifact is reproduced exactly as GCS reports it.
func newReleaseServer(idx releaseIndex) (*httptest.Server, *downloadLog) {
	log := &downloadLog{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		if parts[0] == "blob" {
			arch, pkg, version := parts[1], parts[2], parts[3]
			st := idx[arch][pkg]
			if st == nil || !slices.Contains(st.blobs, version) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			log.record(arch + "/" + pkg)
			rng := r.Header.Get("Range")
			st.recordRange(rng)
			if st.empty {
				return
			}
			if rng == "bytes=0-0" && !st.ignoreRange {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", max(st.size, 1)))
				w.WriteHeader(http.StatusPartialContent)
				st.serve(w, 1)
				return
			}
			st.serve(w, max(st.size, 1))
			return
		}

		// /api/releases/<arch>/packages[/<pkg>/bundles/<version>/download]
		if len(parts) < 4 || parts[0] != "api" || parts[1] != "releases" || parts[3] != "packages" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		arch := parts[2]
		pkgs, ok := idx[arch]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if len(parts) == 4 {
			out := []release.Package{}
			for _, name := range slices.Sorted(maps.Keys(pkgs)) {
				st := pkgs[name]
				pkg := release.Package{Name: name, Latest: release.Version(st.latest)}
				if st.language {
					pkg.Metadata = map[string]string{"language": "true"}
				}
				out = append(out, pkg)
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}

		if len(parts) != 8 || parts[5] != "bundles" || parts[7] != "download" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		name, version := parts[4], parts[6]
		st := pkgs[name]
		if st == nil || !st.hasBundle(version) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"url": cmp.Or(st.signedURL, srv.URL+"/blob/"+arch+"/"+name+"/"+version),
		})
	}))
	return srv, log
}
