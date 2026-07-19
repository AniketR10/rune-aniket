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

// Package multipkg composes several release.Manager backends into one,
// dispatching each package operation to the backend that owns the
// package ID. Git IDs (<host>/<path>, e.g. github.com/owner/repo) route
// to a git-backed manager; everything else routes to the official
// distribution. A single composed manager lets idepkg.Manager and the
// rest of the package machinery stay unaware of where a package comes
// from — including a git package's `requirements`, which resolve
// through the official backend transparently.
package multipkg

import (
	"context"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"unstable.build/go-tui/ide/gitpkg"
)

// Manager is a release.Manager that routes by package ID.
type Manager struct {
	git      release.Manager
	official release.Manager
}

var _ release.Manager = (*Manager)(nil)

// New composes a git-backed manager and the official distribution
// manager into one release.Manager. Git package IDs route to git; all
// other IDs route to official.
func New(git, official release.Manager) *Manager {
	return &Manager{git: git, official: official}
}

// managerFor routes a package ID to its backend.
func (m *Manager) managerFor(pkgID string) release.Manager {
	if gitpkg.IsGitPkgID(pkgID) {
		return m.git
	}
	return m.official
}

// GetPackage satisfies release.Manager.
func (m *Manager) GetPackage(ctx context.Context, pkgID string) (release.Package, error) {
	return m.managerFor(pkgID).GetPackage(ctx, pkgID)
}

// List satisfies release.Manager.
func (m *Manager) List(
	ctx context.Context, pkgID string, filters map[string]string,
) (iterator.Iterator[release.Bundle], error) {
	return m.managerFor(pkgID).List(ctx, pkgID, filters)
}

// Get satisfies release.Manager.
func (m *Manager) Get(
	ctx context.Context, pkgID string, version release.Version,
	pw release.ProgressWriter,
) (release.Bundle, error) {
	return m.managerFor(pkgID).Get(ctx, pkgID, version, pw)
}

// Create satisfies release.Manager.
func (m *Manager) Create(ctx context.Context, pkg release.Package) error {
	return m.managerFor(pkg.Name).Create(ctx, pkg)
}

// UpdatePackageMetadata satisfies release.Manager.
func (m *Manager) UpdatePackageMetadata(
	ctx context.Context, pkgID string, metadata map[string]string,
) error {
	return m.managerFor(pkgID).UpdatePackageMetadata(ctx, pkgID, metadata)
}

// DeletePackage satisfies release.Manager.
func (m *Manager) DeletePackage(ctx context.Context, pkgID string) error {
	return m.managerFor(pkgID).DeletePackage(ctx, pkgID)
}

// Upload satisfies release.Manager.
func (m *Manager) Upload(
	ctx context.Context, bundle release.Bundle, pr release.ProgressReader,
) error {
	return m.managerFor(bundle.Package).Upload(ctx, bundle, pr)
}

// Delete satisfies release.Manager.
func (m *Manager) Delete(
	ctx context.Context, pkgID string, version release.Version,
) error {
	return m.managerFor(pkgID).Delete(ctx, pkgID, version)
}

// ListPackages satisfies release.Manager. GitHub repositories cannot be
// enumerated, so only the official distribution's packages are listed.
func (m *Manager) ListPackages(
	ctx context.Context, filters map[string]string,
) (iterator.Iterator[release.Package], error) {
	return m.official.ListPackages(ctx, filters)
}
