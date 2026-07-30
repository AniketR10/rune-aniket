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
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/cdnrelease"
	"github.com/unstablebuild/ox-api/api/oxapi"

	"unstable.build/go-tui/debug"
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
// as Latest is actually installable, driving the same cdnrelease.Manager
// the editor installs through so the probe cannot diverge from what
// `pkg install <name>` really does.
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
	// A discarding writer takes cdnrelease's IsDiscard fast path and
	// returns the bundle without ever touching the bucket, which is the
	// exact failure this probe exists to catch. Streaming into a real
	// writer forces the signed-URL leg.
	var sink firstChunkWriter
	if _, err := p.manager(rel.arch).Get(ctx, rel.pkg, rel.version, &sink); err != nil &&
		!errors.Is(err, errEnoughRead) {
		return fmt.Errorf("%s: %s: %s", rel.version, leg(err), statusOrError(err))
	}
	if sink.n == 0 {
		return fmt.Errorf("%s: artifact is empty", rel.version)
	}
	return nil
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

// leg names which hop failed. cdnrelease reports a bucket failure as a
// StatusError with no URL and a release-API failure with one, the same
// split idepkg keys its user-facing errors off.
func leg(err error) string {
	if se, ok := errors.AsType[*cdnrelease.StatusError](err); ok && se.URL == "" {
		return "artifact"
	}
	return "bundle"
}

// statusOrError keeps signed URLs out of pages: cdnrelease renders the
// full request URL in StatusError.Error, and those carry credentials.
func statusOrError(err error) string {
	if se, ok := errors.AsType[*cdnrelease.StatusError](err); ok {
		return fmt.Sprintf("status %d", se.Status)
	}
	return err.Error()
}

// errEnoughRead stops cdnrelease's streaming copy once any bytes have
// arrived. Artifacts run to hundreds of megabytes and the probe runs
// every minute, so reading the first chunk is all the proof the bucket
// object is readable that is worth paying for.
var errEnoughRead = errors.New("read enough")

type firstChunkWriter struct{ n int64 }

func (w *firstChunkWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), errEnoughRead
}

func (w *firstChunkWriter) Progress(int64, int64, string) {}

var _ release.ProgressWriter = (*firstChunkWriter)(nil)
