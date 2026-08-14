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

package idescavenger_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/go-tui/ide/idescavenger"
)

func uri(t *testing.T, path string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)
	return u
}

type fixture struct {
	cleaner *idescavenger.Cleaner
	storage storageapi.Service
	missing map[string]struct{}
	open    []workspaceapi.URI
	openErr error
	cleaned []string
	hookErr error
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		storage: storagestub.NewInMemoryService(),
		missing: make(map[string]struct{}),
	}
	cleaner, err := idescavenger.New(idescavenger.Config{
		Storage: f.storage,
		OpenWorkspaces: func(context.Context) ([]workspaceapi.URI, error) {
			return f.open, f.openErr
		},
		Stat: func(name string) (fs.FileInfo, error) {
			if _, gone := f.missing[name]; gone {
				return nil, &fs.PathError{
					Op: "stat", Path: name, Err: fs.ErrNotExist,
				}
			}
			return nil, nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleaner.Close() })

	cleaner.AddWorkspaceHook(func(
		_ context.Context, cwd workspaceapi.URI,
	) error {
		if f.hookErr != nil {
			return f.hookErr
		}
		f.cleaned = append(f.cleaned, cwd.String())
		return nil
	})
	f.cleaner = cleaner
	return f
}

func TestRunOnce(t *testing.T) {
	ctx := context.Background()

	t.Run("cleans workspaces that no longer exist", func(t *testing.T) {
		f := newFixture(t)
		gone := uri(t, "/tmp/gone")
		alive := uri(t, "/tmp/alive")
		f.missing[gone.Path()] = struct{}{}
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, gone))
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, alive))

		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Equal(t, []string{gone.String()}, f.cleaned)

		// the cleaned workspace is forgotten, so a second pass is a no-op
		f.cleaned = nil
		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Empty(t, f.cleaned)
	})

	t.Run("keeps workspaces that are open", func(t *testing.T) {
		f := newFixture(t)
		gone := uri(t, "/tmp/gone")
		f.missing[gone.Path()] = struct{}{}
		f.open = []workspaceapi.URI{gone}
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, gone))

		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Empty(t, f.cleaned)
	})

	t.Run("abandons the pass when open workspaces are unknown", func(t *testing.T) {
		f := newFixture(t)
		gone := uri(t, "/tmp/gone")
		f.missing[gone.Path()] = struct{}{}
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, gone))
		f.openErr = errors.New("event loop is gone")

		require.Error(t, f.cleaner.RunOnce(ctx))
		assert.Empty(t, f.cleaned)
	})

	t.Run("retries a workspace whose hook failed", func(t *testing.T) {
		f := newFixture(t)
		gone := uri(t, "/tmp/gone")
		f.missing[gone.Path()] = struct{}{}
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, gone))

		f.hookErr = errors.New("storage is busy")
		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Empty(t, f.cleaned)

		f.hookErr = nil
		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Equal(t, []string{gone.String()}, f.cleaned)
	})

	t.Run("ignores workspaces that are not local", func(t *testing.T) {
		f := newFixture(t)
		remote, err := workspaceapi.ParseURI("ssh://host/tmp/gone")
		require.NoError(t, err)
		f.missing["/tmp/gone"] = struct{}{}
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, remote))

		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Empty(t, f.cleaned)
	})

	t.Run("leaves a workspace it cannot stat", func(t *testing.T) {
		f := newFixture(t)
		unreachable := uri(t, "/tmp/unreachable")
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, unreachable))

		require.NoError(t, f.cleaner.RunOnce(ctx))
		assert.Empty(t, f.cleaned)
	})
}

func TestRegisterNewWorkspaceIsIdempotent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	gone := uri(t, "/tmp/gone")
	f.missing[gone.Path()] = struct{}{}

	for range 3 {
		require.NoError(t, f.cleaner.RegisterNewWorkspace(ctx, gone))
	}
	require.NoError(t, f.cleaner.Seed(ctx, []workspaceapi.URI{gone}))

	require.NoError(t, f.cleaner.RunOnce(ctx))
	assert.Equal(t, []string{gone.String()}, f.cleaned)
}

// The default Stat must recognize a workspace that really is gone.
func TestRunOnceWithRealFilesystem(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	gone := uri(t, filepath.Join(dir, "gone"))
	alive := uri(t, filepath.Join(dir, "alive"))
	require.NoError(t, os.Mkdir(alive.Path(), 0o755))

	cleaner, err := idescavenger.New(idescavenger.Config{
		Storage: storagestub.NewInMemoryService(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleaner.Close() })

	var cleaned []string
	cleaner.AddWorkspaceHook(func(
		_ context.Context, cwd workspaceapi.URI,
	) error {
		cleaned = append(cleaned, cwd.String())
		return nil
	})
	require.NoError(t, cleaner.RegisterNewWorkspace(ctx, gone))
	require.NoError(t, cleaner.RegisterNewWorkspace(ctx, alive))

	require.NoError(t, cleaner.RunOnce(ctx))
	assert.Equal(t, []string{gone.String()}, cleaned)
}
