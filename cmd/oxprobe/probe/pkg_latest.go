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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/cdnrelease"
	"github.com/unstablebuild/ox-api/api/oxapi"

	"unstable.build/rune/debug"
)

// pkgLatestConcurrency bounds the in-flight artifact checks so a wide
// registry still finishes inside the per-probe timeout without opening
// one connection per package at once.
const pkgLatestConcurrency = 8

// languageMetadataKey marks the tree-sitter grammar packages, which we
// republish wholesale rather than build.
const languageMetadataKey = "language"

// ownedLanguages mirrors OWNED_LANGUAGES in deploy/package_versions.sh:
// grammar packages are excluded from the owned set, except these
// toolchains, which we build and release ourselves.
var ownedLanguages = []string{"go", "python", "rust"}

// PkgLatestProbe asserts that the version each package we own advertises
// as Latest is actually installable, walking the same list -> bundle ->
// artifact legs `pkg install <name>` walks, against the same URL shapes,
// so the probe cannot diverge from what a real install does.
//
// The release index, the bundle record, and the bucket object are owned
// by different systems, so a Latest pointer can outlive its artifact — a
// publish that registers bundle metadata but never uploads the tarball
// leaves `pkg install <name>` broken for every client on that arch while
// DNS, TLS, oauth, and the deep /health report all stay green.
//
// The owned set is discovered per run rather than pinned, so a package
// published after this code was written is covered the moment it lands.
// It is the same set deploy/package_versions.sh reports.
type PkgLatestProbe struct {
	APIURL     *url.URL
	Archs      []string
	IsCritical bool
	Client     *http.Client
}

// Layer implements Probe.
func (p PkgLatestProbe) Layer() string { return "pkg_latest" }

// Critical implements Probe.
func (p PkgLatestProbe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p PkgLatestProbe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, "pkg_latest", p.IsCritical, func(ctx context.Context) (string, error) {
		releases, names, err := p.ownedReleases(ctx)
		if err != nil {
			return "", err
		}
		// A registry advertising nothing installable is itself an
		// outage, and would otherwise pass as zero targets checked.
		if len(releases) == 0 {
			return "", errors.New("no owned package advertises a Latest version")
		}

		// Every target is reported, not just the first failure, so one
		// run tells on-call exactly which arch/package pairs are broken.
		failures := make([]string, len(releases))
		sem := make(chan struct{}, pkgLatestConcurrency)
		var wg sync.WaitGroup
		for i, rel := range releases {
			wg.Add(1)
			go debug.CapturePanicReport(func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if err := p.verify(ctx, rel); err != nil {
					failures[i] = fmt.Sprintf("%s/%s: %v", rel.arch, rel.pkg, err)
				}
			})
		}
		wg.Wait()

		var broken []string
		for _, f := range failures {
			if f != "" {
				broken = append(broken, f)
			}
		}
		if len(broken) > 0 {
			return "", fmt.Errorf("%d/%d latest versions not downloadable: %s",
				len(broken), len(releases), strings.Join(broken, "; "))
		}
		return fmt.Sprintf("%d latest versions downloadable (%d packages, %d archs)",
			len(releases), names, len(p.Archs)), nil
	})
}

type pkgRelease struct {
	arch    string
	pkg     string
	version release.Version
}

// ownedReleases resolves the arch/package/Latest rows to verify using the
// same ownership rules as deploy/package_versions.sh, and reports how
// many distinct packages those rows cover.
func (p PkgLatestProbe) ownedReleases(ctx context.Context) ([]pkgRelease, int, error) {
	archs := slices.Sorted(slices.Values(p.Archs))
	listed := make([][]release.Package, len(archs))
	for i, arch := range archs {
		pkgs, err := p.listPackages(ctx, arch)
		if err != nil {
			return nil, 0, fmt.Errorf("list %s: %s", arch, statusOrError(err))
		}
		listed[i] = pkgs
	}

	// Only one arch's registry carries the language marker, so a name
	// counts as a grammar when any arch flags it. Without the union the
	// unmarked archs drag all ~300 grammars into the owned set.
	grammars := map[string]bool{}
	for _, pkgs := range listed {
		for _, pkg := range pkgs {
			if pkg.Metadata[languageMetadataKey] == "true" {
				grammars[pkg.Name] = true
			}
		}
	}

	var out []pkgRelease
	names := map[string]bool{}
	for i, arch := range archs {
		for _, pkg := range listed[i] {
			if grammars[pkg.Name] && !slices.Contains(ownedLanguages, pkg.Name) {
				continue
			}
			// An empty Latest means the package is not published for
			// this arch, which is routine rather than an outage.
			if pkg.Latest == "" {
				continue
			}
			names[pkg.Name] = true
			out = append(out, pkgRelease{arch: arch, pkg: pkg.Name, version: pkg.Latest})
		}
	}
	slices.SortFunc(out, func(a, b pkgRelease) int {
		if c := strings.Compare(a.arch, b.arch); c != 0 {
			return c
		}
		return strings.Compare(a.pkg, b.pkg)
	})
	return out, len(names), nil
}

func (p PkgLatestProbe) listPackages(ctx context.Context, arch string) ([]release.Package, error) {
	it, err := p.manager(arch).ListPackages(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = it.Close() }()
	var out []release.Package
	for {
		pkg, ok := it.Next(ctx)
		if !ok {
			break
		}
		out = append(out, pkg)
	}
	return out, it.Err()
}

func (p PkgLatestProbe) verify(ctx context.Context, rel pkgRelease) error {
	signed, err := p.downloadURL(ctx, rel)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	if err != nil {
		return err
	}
	// HEAD is not an option: the URL is signed for GET. A ranged GET is
	// the only way to prove the bucket object is readable without paying
	// egress for the whole artifact every minute.
	req.Header.Set("Range", "bytes=0-0")
	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: artifact: %s", rel.version, statusOrError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("%s: artifact: status %d", rel.version, resp.StatusCode)
	}

	// Bounded regardless of whether the backend honoured Range, so a
	// bucket that ignores it cannot silently restore the full download.
	n, err := io.CopyN(io.Discard, resp.Body, 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: artifact: %s", rel.version, statusOrError(err))
	}
	if n == 0 {
		return fmt.Errorf("%s: artifact is empty", rel.version)
	}
	return nil
}

// downloadURL resolves the signed artifact URL for rel. The artifact leg
// deliberately bypasses cdnrelease.Manager: signed URLs must not receive
// credentials, and Manager.Get issues an unranged GET with no way to
// bound the read, so aborting it still bills the full object. We give up
// "drives the exact client the editor installs with" on the artifact hop
// only; the failure this probe exists to catch — a Latest pointer whose
// bucket object is missing — is fully covered by a ranged GET.
func (p PkgLatestProbe) downloadURL(ctx context.Context, rel pkgRelease) (string, error) {
	u := fmt.Sprintf("%s/packages/%s/bundles/%s/download",
		releasesURL(p.APIURL, rel.arch), url.PathEscape(rel.pkg), url.PathEscape(string(rel.version)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: bundle: %s", rel.version, statusOrError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: bundle: status %d", rel.version, resp.StatusCode)
	}
	// encoding/json matches "url" and "URL" against the same field.
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("%s: bundle: %s", rel.version, err)
	}
	if body.URL == "" {
		return "", fmt.Errorf("%s: bundle: no url", rel.version)
	}
	return body.URL, nil
}

func (p PkgLatestProbe) manager(arch string) release.Manager {
	return cdnrelease.NewManager(p.Client, releasesURL(p.APIURL, arch))
}

// releasesURL mirrors idepkg.ReleasesURL. It is duplicated rather than
// imported because idepkg drags the whole editor (down to the GUI
// toolkit) into this headless probe binary; a test pins the two to the
// same shape.
func releasesURL(api *url.URL, arch string) string {
	return strings.TrimSuffix(api.String(), "/") + "/api/releases/" + arch
}

// statusOrError keeps signed URLs out of pages: cdnrelease renders the
// full request URL in StatusError.Error and net/http renders it in
// url.Error, and those carry credentials.
func statusOrError(err error) string {
	if se, ok := errors.AsType[*cdnrelease.StatusError](err); ok {
		return fmt.Sprintf("status %d", se.Status)
	}
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Err.Error()
	}
	return err.Error()
}
