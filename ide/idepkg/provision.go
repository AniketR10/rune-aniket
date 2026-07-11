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

package idepkg

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
)

// ProvisionEntry is a single package/version pair to install on a remote
// workspace server so its language toolchain matches the local IDE.
type ProvisionEntry struct {
	ID      string
	Version string
}

// provisionTokenRe restricts every character of the encoded manifest to a
// shell-safe set. Package ids and versions are already escapeString-normalized
// on disk; this is defense-in-depth against a hand-crafted --install value
// that the remote shell would otherwise reparse.
var provisionTokenRe = regexp.MustCompile(`^[A-Za-z0-9._@,%+~/-]+$`)

// provisionManifestSource is the subset of *Manager that
// BuildProvisionManifest needs, kept small so it can be faked in tests.
type provisionManifestSource interface {
	ListInstalledPackages(ctx context.Context) (iterator.Iterator[string], error)
	PackageVersionInUse(pkgID string) (release.Version, bool)
}

// BuildProvisionManifest returns the in-use version of every installed
// package so a remote workspace server can mirror the local toolchain.
// Packages with no in-use version are skipped.
func BuildProvisionManifest(
	ctx context.Context, pkg provisionManifestSource,
) ([]ProvisionEntry, error) {
	it, err := pkg.ListInstalledPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installed packages: %w", err)
	}
	ids, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return nil, fmt.Errorf("collect installed packages: %w", err)
	}
	var entries []ProvisionEntry
	for _, id := range ids {
		version, ok := pkg.PackageVersionInUse(id)
		if !ok {
			continue
		}
		entries = append(entries, ProvisionEntry{ID: id, Version: string(version)})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

// EncodeProvisionManifest renders entries as an `id@ver,id2@ver` CSV token.
// Entries with characters outside the shell-safe class are dropped so the
// value stays a single validated token; an empty result encodes to "".
func EncodeProvisionManifest(entries []ProvisionEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.ID == "" || e.Version == "" {
			continue
		}
		token := e.ID + "@" + e.Version
		if !provisionTokenRe.MatchString(token) {
			log.Warnf("provision: skipping package with unsafe id/version %q@%q",
				e.ID, e.Version)
			continue
		}
		parts = append(parts, token)
	}
	return strings.Join(parts, ",")
}

// ParseProvisionManifest parses an `id@ver,id2@ver` CSV token back into
// entries. A manifest containing characters outside the shell-safe class is
// rejected wholesale, and individually malformed entries are logged and
// skipped, so a corrupt manifest can never abort remote startup.
func ParseProvisionManifest(manifest string) []ProvisionEntry {
	manifest = strings.TrimSpace(manifest)
	if manifest == "" {
		return nil
	}
	if !provisionTokenRe.MatchString(manifest) {
		log.Warnf("provision: rejecting manifest with unsafe characters")
		return nil
	}
	var entries []ProvisionEntry
	for token := range strings.SplitSeq(manifest, ",") {
		if token == "" {
			continue
		}
		at := strings.LastIndex(token, "@")
		if at <= 0 || at == len(token)-1 {
			log.Warnf("provision: skipping malformed manifest entry %q", token)
			continue
		}
		entries = append(entries, ProvisionEntry{
			ID:      token[:at],
			Version: token[at+1:],
		})
	}
	return entries
}
