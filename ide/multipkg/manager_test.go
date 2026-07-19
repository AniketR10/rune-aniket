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

package multipkg

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
)

// recordingManager records which package IDs it was called with and
// returns a package tagged with its own name so routing is observable.
type recordingManager struct {
	name string
	got  []string
}

func (r *recordingManager) GetPackage(_ context.Context, pkgID string) (release.Package, error) {
	r.got = append(r.got, pkgID)
	return release.Package{Name: r.name}, nil
}

func (r *recordingManager) List(
	_ context.Context, pkgID string, _ map[string]string,
) (iterator.Iterator[release.Bundle], error) {
	r.got = append(r.got, pkgID)
	return iterator.FromSlice([]release.Bundle{{Package: r.name}}), nil
}

func (r *recordingManager) Get(
	_ context.Context, pkgID string, _ release.Version, _ release.ProgressWriter,
) (release.Bundle, error) {
	r.got = append(r.got, pkgID)
	return release.Bundle{Package: r.name}, nil
}

func (r *recordingManager) Create(_ context.Context, pkg release.Package) error {
	r.got = append(r.got, pkg.Name)
	return nil
}

func (r *recordingManager) UpdatePackageMetadata(
	_ context.Context, pkgID string, _ map[string]string,
) error {
	r.got = append(r.got, pkgID)
	return nil
}

func (r *recordingManager) DeletePackage(_ context.Context, pkgID string) error {
	r.got = append(r.got, pkgID)
	return nil
}

func (r *recordingManager) Upload(
	_ context.Context, bundle release.Bundle, _ release.ProgressReader,
) error {
	r.got = append(r.got, bundle.Package)
	return nil
}

func (r *recordingManager) Delete(
	_ context.Context, pkgID string, _ release.Version,
) error {
	r.got = append(r.got, pkgID)
	return nil
}

func (r *recordingManager) ListPackages(
	_ context.Context, _ map[string]string,
) (iterator.Iterator[release.Package], error) {
	return iterator.FromSlice([]release.Package{{Name: r.name}}), nil
}

func TestRouting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	newM := func() (*Manager, *recordingManager, *recordingManager) {
		gitmgr := &recordingManager{name: "git"}
		official := &recordingManager{name: "official"}
		return New(gitmgr, official), gitmgr, official
	}

	t.Run("git IDs route to the git manager", func(t *testing.T) {
		t.Parallel()
		for _, id := range []string{
			"github.com/owner/repo",
			"gitlab.com/o/r",
			"gitlab.com/group/subgroup/repo",
			"git.example.com/o/r",
		} {
			m, gitmgr, official := newM()
			pkg, err := m.GetPackage(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, "git", pkg.Name, "id=%q", id)
			assert.Equal(t, []string{id}, gitmgr.got, "id=%q", id)
			assert.Empty(t, official.got, "id=%q", id)
		}
	})

	t.Run("non-git IDs route to the official manager", func(t *testing.T) {
		t.Parallel()
		for _, id := range []string{
			"go", "python", "github.com/owner", "localhost/o/r",
		} {
			m, gitmgr, official := newM()
			pkg, err := m.GetPackage(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, "official", pkg.Name, "id=%q", id)
			assert.Equal(t, []string{id}, official.got, "id=%q", id)
			assert.Empty(t, gitmgr.got, "id=%q", id)
		}
	})

	t.Run("Get and List route by ID", func(t *testing.T) {
		t.Parallel()
		m, gitmgr, official := newM()
		_, err := m.Get(ctx, "github.com/o/r", "abc", release.NopProgressWriter(io.Discard))
		require.NoError(t, err)
		_, err = m.List(ctx, "go", nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"github.com/o/r"}, gitmgr.got)
		assert.Equal(t, []string{"go"}, official.got)
	})

	t.Run("Create/Upload route by package name", func(t *testing.T) {
		t.Parallel()
		m, gitmgr, official := newM()
		require.NoError(t, m.Create(ctx, release.Package{Name: "github.com/o/r"}))
		require.NoError(t, m.Upload(ctx, release.Bundle{Package: "go"}, nil))
		assert.Equal(t, []string{"github.com/o/r"}, gitmgr.got)
		assert.Equal(t, []string{"go"}, official.got)
	})

	t.Run("ListPackages lists only the official distribution", func(t *testing.T) {
		t.Parallel()
		m, _, _ := newM()
		it, err := m.ListPackages(ctx, nil)
		require.NoError(t, err)
		pkgs, err := iterator.ToSlice(ctx, it)
		require.NoError(t, err)
		require.Len(t, pkgs, 1)
		assert.Equal(t, "official", pkgs[0].Name)
	})
}
