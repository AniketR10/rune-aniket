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
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/unstablebuild/ox-api/api/oxapi"
)

// Resolver abstracts DNS resolution so the dns probe can be tested with
// an injected fake. *net.Resolver satisfies it.
type Resolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

// TLSDialer abstracts establishing a TLS connection so the tls probe can
// be tested with an injected fake.
type TLSDialer interface {
	DialTLS(ctx context.Context, addr string) (*tls.ConnectionState, error)
}

// DNSProbe resolves a host and asserts it matches an expected static IP.
type DNSProbe struct {
	LayerName  string
	Host       string
	ExpectedIP string
	IsCritical bool
	Resolver   Resolver
}

// Layer implements Probe.
func (p DNSProbe) Layer() string { return p.LayerName }

// Critical implements Probe.
func (p DNSProbe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p DNSProbe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, p.LayerName, p.IsCritical, func(ctx context.Context) (string, error) {
		addrs, err := p.Resolver.LookupHost(ctx, p.Host)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", p.Host, err)
		}
		if p.ExpectedIP == "" {
			return fmt.Sprintf("%s -> %v", p.Host, addrs), nil
		}
		for _, a := range addrs {
			if a == p.ExpectedIP {
				return fmt.Sprintf("%s -> %s", p.Host, a), nil
			}
		}
		sort.Strings(addrs)
		return "", fmt.Errorf("%s resolved to %v, want %s", p.Host, addrs, p.ExpectedIP)
	})
}

// TLSProbe handshakes against an address and fails if the leaf
// certificate expires within MinValidity.
type TLSProbe struct {
	LayerName   string
	Addr        string
	MinValidity time.Duration
	IsCritical  bool
	Dialer      TLSDialer
	Now         func() time.Time
}

// Layer implements Probe.
func (p TLSProbe) Layer() string { return p.LayerName }

// Critical implements Probe.
func (p TLSProbe) Critical() bool { return p.IsCritical }

// Run implements Probe.
func (p TLSProbe) Run(ctx context.Context) oxapi.CheckResult {
	return run(ctx, p.LayerName, p.IsCritical, func(ctx context.Context) (string, error) {
		state, err := p.Dialer.DialTLS(ctx, p.Addr)
		if err != nil {
			return "", fmt.Errorf("tls dial %s: %w", p.Addr, err)
		}
		if len(state.PeerCertificates) == 0 {
			return "", fmt.Errorf("tls %s: no peer certificates", p.Addr)
		}
		now := time.Now
		if p.Now != nil {
			now = p.Now
		}
		leaf := state.PeerCertificates[0]
		remaining := leaf.NotAfter.Sub(now())
		if remaining < p.MinValidity {
			return "", fmt.Errorf("tls %s: cert expires in %s (< %s)",
				p.Addr, remaining.Truncate(time.Hour), p.MinValidity)
		}
		return fmt.Sprintf("cert valid for %s", remaining.Truncate(time.Hour)), nil
	})
}

// NetTLSDialer is the production TLSDialer backed by crypto/tls.
type NetTLSDialer struct {
	Timeout time.Duration
}

// DialTLS implements TLSDialer using crypto/tls.
func (d NetTLSDialer) DialTLS(ctx context.Context, addr string) (*tls.ConnectionState, error) {
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: d.Timeout}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	state := conn.(*tls.Conn).ConnectionState()
	return &state, nil
}
