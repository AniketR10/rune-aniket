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
package workspace

import (
	"context"

	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

func assertIteratorEqual(
	t *testing.T, expected []string, it iterator.Iterator[string],
) {
	var actual []string
	for {
		next, ok := it.Next()
		if !ok {
			break
		}
		actual = append(actual, next)
	}
	require.NoError(t, it.Err())

	sort.Strings(actual)
	sort.Strings(expected)
	assert.Equal(t, actual, expected)
}

func TestListFiles(t *testing.T) {
	t.Run("lists all files under workspace as relative", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		for _, path := range []string{".", dir} {
			t.Run(path, func(t *testing.T) {
				uri, err := workspaceapi.CurrentUserHostURI(dir)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
				require.NoError(t, err)

				_, err = os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE, 0666)
				require.NoError(t, err)

				scheme, err := NewFileScheme(context.Background(), config.NopConfig(), uri)
				require.NoError(t, err)

				it, err := ListFiles(context.Background(), scheme, path)
				assertIteratorEqual(t, []string{"a", "b"}, it)
			})
		}
	})

	t.Run("lists all files under non-workspace dir as absolute", func(t *testing.T) {
		workspaceDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		uri, err := workspaceapi.CurrentUserHostURI(workspaceDir)
		require.NoError(t, err)

		f1, err := os.OpenFile(filepath.Join(dir, "a"), os.O_CREATE, 0666)
		require.NoError(t, err)

		f2, err := os.OpenFile(filepath.Join(dir, "b"), os.O_CREATE, 0666)
		require.NoError(t, err)

		scheme, err := NewFileScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		it, err := ListFiles(context.Background(), scheme, dir)
		assertIteratorEqual(t, []string{f1.Name(), f2.Name()}, it)
	})
}
