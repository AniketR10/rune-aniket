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
