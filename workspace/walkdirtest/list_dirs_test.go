// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package walkdir

import (
	"context"
	"strconv"

	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/walkdir"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"go.uber.org/goleak"
	"unstable.build/go-tui/workspace"
)

func TestListDirs(t *testing.T) {

	t.Run("lists all dirs under workspace as relative", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		for _, path := range []string{".", dir} {
			t.Run(path, func(t *testing.T) {
				uri, err := workspaceapi.CurrentUserHostURI(dir)
				require.NoError(t, err)

				err = os.MkdirAll(filepath.Join(dir, "a"), 0666)
				require.NoError(t, err)

				err = os.MkdirAll(filepath.Join(dir, "b"), 0666)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "c"), os.O_CREATE, 0666)
				require.NoError(t, err)

				scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
				require.NoError(t, err)

				it, err := walkdir.ListDirs(context.Background(), scheme, path)
				assertIteratorEqual(t, []string{"a", "b"}, it)

				require.NoError(t, scheme.Close())
			})
		}
	})
	t.Run("lists nested dirs", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp(".", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		err = os.Mkdir(filepath.Join(dir, "a"), 0777)
		require.NoError(t, err)

		err = os.Mkdir(filepath.Join(dir, "a", "b"), 0777)
		require.NoError(t, err)

		err = os.Mkdir(filepath.Join(dir, "a", "b", "c"), 0777)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListDirs(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{"a/b/c", "a", "a/b"}, it)

		require.NoError(t, scheme.Close())
	})

	t.Run("Close before scanning all should abort", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		uri, err := workspaceapi.CurrentUserHostURI(dir)
		require.NoError(t, err)

		const n = 1000
		for i := range n {
			subdir := filepath.Join(dir, strconv.Itoa(i))
			require.NoError(t, os.MkdirAll(subdir, 0777))
			err = os.MkdirAll(filepath.Join(subdir, "a"), 0666)
			require.NoError(t, err)
		}

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListDirs(context.Background(), scheme, dir)
		require.NoError(t, it.Close())

		require.NoError(t, scheme.Close())
	})

	t.Run("lists all files under non-workspace dir as absolute", func(t *testing.T) {
		defer goleak.VerifyNone(t)

		workspaceDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(workspaceDir)
		})

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		uri, err := workspaceapi.CurrentUserHostURI(workspaceDir)
		require.NoError(t, err)

		dir1 := filepath.Join(dir, "a")
		err = os.MkdirAll(dir1, 0666)
		require.NoError(t, err)

		dir2 := filepath.Join(dir, "b")
		err = os.MkdirAll(dir2, 0666)
		require.NoError(t, err)

		scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := walkdir.ListDirs(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{dir1, dir2}, it)
		require.NoError(t, scheme.Close())
	})
}
