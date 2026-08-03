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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecentLabels(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []recentEntry
	}{
		{
			name: "distinct basenames use the directory name",
			in:   []string{"/home/me/alpha", "/home/me/beta"},
			want: []recentEntry{
				{label: "alpha", path: "/home/me/alpha"},
				{label: "beta", path: "/home/me/beta"},
			},
		},
		{
			name: "collision walks up one segment",
			in:   []string{"/home/app/web", "/home/api/web"},
			want: []recentEntry{
				{label: "app/web", path: "/home/app/web"},
				{label: "api/web", path: "/home/api/web"},
			},
		},
		{
			name: "three-way collision walks up until distinct",
			in:   []string{"/a/x/web", "/b/x/web", "/b/y/web"},
			want: []recentEntry{
				{label: "a/x/web", path: "/a/x/web"},
				{label: "b/x/web", path: "/b/x/web"},
				{label: "y/web", path: "/b/y/web"},
			},
		},
		{
			name: "exact duplicates are collapsed keeping the first",
			in:   []string{"/home/me/proj", "/home/me/proj"},
			want: []recentEntry{
				{label: "proj", path: "/home/me/proj"},
			},
		},
		{
			name: "file uri and trailing slash normalize to the same path",
			in:   []string{"file:///home/me/proj/", "/home/me/proj"},
			want: []recentEntry{
				{label: "proj", path: "/home/me/proj"},
			},
		},
		{
			name: "blank entries are dropped",
			in:   []string{"", "  ", "/home/me/proj"},
			want: []recentEntry{
				{label: "proj", path: "/home/me/proj"},
			},
		},
		{
			name: "remote uris are left intact",
			in:   []string{"ssh://host/srv/proj"},
			want: []recentEntry{
				{label: "ssh://host/srv/proj", path: "ssh://host/srv/proj"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, recentLabels(tt.in))
		})
	}
}

func TestRecentLabelsEmpty(t *testing.T) {
	assert.Empty(t, recentLabels(nil))
}

func TestNormalizeWorkspacePath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/home/me/proj", "/home/me/proj"},
		{"/home/me/proj/", "/home/me/proj"},
		{"file:///home/me/proj", "/home/me/proj"},
		{"  /home/me/proj  ", "/home/me/proj"},
		{"ssh://host/srv", "ssh://host/srv"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeWorkspacePath(tt.in))
		})
	}
}

// TestRecentWorkspacesRoundTrip asserts recorded paths persist and read
// back newest-first and de-duplicated.
func TestRecentWorkspacesRoundTrip(t *testing.T) {
	storage := newRuneStorage(t.TempDir())
	t.Cleanup(func() { _ = storage.Close() })
	r := newRecentWorkspaces(storage)

	r.record("/a")
	r.record("/b")
	r.record("/a") // re-opening moves it back to the front
	r.record("")   // ignored

	require.Equal(t, []string{"/a", "/b"}, r.paths())
}

func TestRecentWorkspacesEmpty(t *testing.T) {
	storage := newRuneStorage(t.TempDir())
	t.Cleanup(func() { _ = storage.Close() })
	assert.Nil(t, newRecentWorkspaces(storage).paths())
}
