// Copyright (C) 2017-2026 The Rune Authors
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

package webfetch

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid https URL",
			url:  "https://example.com/page",
		},
		{
			name: "valid http URL",
			url:  "http://example.com/page",
		},
		{
			name:    "ftp scheme blocked",
			url:     "ftp://example.com",
			wantErr: true,
			errMsg:  "unsupported scheme",
		},
		{
			name:    "file scheme blocked",
			url:     "file:///etc/passwd",
			wantErr: true,
			errMsg:  "unsupported scheme",
		},
		{
			name:    "localhost blocked",
			url:     "http://localhost/secret",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    "localhost uppercase blocked",
			url:     "http://LOCALHOST/secret",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    ".local suffix blocked",
			url:     "http://myhost.local/path",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    ".internal suffix blocked",
			url:     "http://service.internal/api",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    ".localhost suffix blocked",
			url:     "http://app.localhost/api",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    "metadata.google.internal blocked",
			url:     "http://metadata.google.internal/v1",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    "cloud metadata IP blocked",
			url:     "http://169.254.169.254/latest",
			wantErr: true,
			errMsg:  "blocked hostname",
		},
		{
			name:    "empty hostname",
			url:     "http:///path",
			wantErr: true,
			errMsg:  "empty hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url, false)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{"loopback v4", "127.0.0.1", true},
		{"loopback v4 other", "127.0.0.2", true},
		{"loopback v6", "::1", true},
		{"private 10.x", "10.0.0.1", true},
		{"private 172.16.x", "172.16.0.1", true},
		{"private 172.31.x", "172.31.255.255", true},
		{"not private 172.15.x", "172.15.0.1", false},
		{"private 192.168.x", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"zero network", "0.0.0.1", true},
		{"public IP", "8.8.8.8", false},
		{"public IP 2", "93.184.216.34", false},
		{"ULA v6", "fd00::1", true},
		{"link-local v6", "fe80::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.NotNil(t, ip, "failed to parse %s", tt.ip)
			assert.Equal(t, tt.blocked, isBlockedIP(ip))
		})
	}
}

func TestIsRedirect(t *testing.T) {
	assert.True(t, isRedirect(301))
	assert.True(t, isRedirect(302))
	assert.True(t, isRedirect(303))
	assert.True(t, isRedirect(307))
	assert.True(t, isRedirect(308))
	assert.False(t, isRedirect(200))
	assert.False(t, isRedirect(404))
}

func TestResolveRedirect(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		location string
		want     string
	}{
		{
			name:     "absolute",
			base:     "https://a.com/page",
			location: "https://b.com/other",
			want:     "https://b.com/other",
		},
		{
			name:     "relative",
			base:     "https://a.com/dir/page",
			location: "/new",
			want:     "https://a.com/new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRedirect(tt.base, tt.location)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
