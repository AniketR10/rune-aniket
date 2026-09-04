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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/api/oxapi"
)

type fakeResolver struct {
	addrs []string
	err   error
}

func (f fakeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return f.addrs, f.err
}

func TestDNSProbe(t *testing.T) {
	tests := []struct {
		name     string
		resolver fakeResolver
		expectIP string
		wantFail bool
	}{
		{name: "matches expected ip", resolver: fakeResolver{addrs: []string{"1.2.3.4"}}, expectIP: "1.2.3.4"},
		{name: "no expectation resolves", resolver: fakeResolver{addrs: []string{"9.9.9.9"}}, expectIP: ""},
		{name: "wrong ip fails", resolver: fakeResolver{addrs: []string{"9.9.9.9"}}, expectIP: "1.2.3.4", wantFail: true},
		{name: "resolve error fails", resolver: fakeResolver{err: errors.New("nxdomain")}, expectIP: "", wantFail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := DNSProbe{LayerName: "dns", Host: "api.example", ExpectedIP: tc.expectIP, Resolver: tc.resolver}
			res := p.Run(context.Background())
			if tc.wantFail {
				require.Equal(t, oxapi.CheckFail, res.Status, res.Detail)
			} else {
				require.Equal(t, oxapi.CheckOK, res.Status, res.Detail)
			}
		})
	}
}

type fakeDialer struct {
	state *tls.ConnectionState
	err   error
}

func (f fakeDialer) DialTLS(ctx context.Context, addr string) (*tls.ConnectionState, error) {
	return f.state, f.err
}

func stateExpiringIn(d time.Duration, now time.Time) *tls.ConnectionState {
	return &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{{NotAfter: now.Add(d)}},
	}
}

func TestTLSProbe(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }

	tests := []struct {
		name     string
		dialer   fakeDialer
		minValid time.Duration
		wantFail bool
	}{
		{name: "valid", dialer: fakeDialer{state: stateExpiringIn(30*24*time.Hour, now)}, minValid: 14 * 24 * time.Hour},
		{name: "expiring soon", dialer: fakeDialer{state: stateExpiringIn(2*24*time.Hour, now)}, minValid: 14 * 24 * time.Hour, wantFail: true},
		{name: "dial error", dialer: fakeDialer{err: errors.New("refused")}, minValid: time.Hour, wantFail: true},
		{name: "no certs", dialer: fakeDialer{state: &tls.ConnectionState{}}, minValid: time.Hour, wantFail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := TLSProbe{LayerName: "tls", Addr: "api.example:443", MinValidity: tc.minValid, Dialer: tc.dialer, Now: nowFn}
			res := p.Run(context.Background())
			if tc.wantFail {
				require.Equal(t, oxapi.CheckFail, res.Status, res.Detail)
			} else {
				require.Equal(t, oxapi.CheckOK, res.Status, res.Detail)
			}
		})
	}
}
