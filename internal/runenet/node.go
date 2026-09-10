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

// Package runenet embeds a userspace WireGuard node in the Rune
// process so a user's machines form a private mesh. It needs no daemon
// and no root: peers reach each other directly, authenticated by their
// WireGuard identity, and the mesh exposes standard net.Listener and
// net.Conn so the existing workspace RPC plumbing runs over it
// unchanged.
package runenet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
	"unstable.build/rune/internal/debug"
)

// ErrNotStarted is returned by every method that needs a live mesh
// node while the node is not running.
var ErrNotStarted = errors.New("network node is not running")

// ErrPeerNotFound is returned when a rune:// URI names a machine that
// is not in the tailnet.
var ErrPeerNotFound = errors.New("peer not found in network")

// ErrNotAuthenticated is returned when joining the mesh needs a Rune
// account and none is signed in.
var ErrNotAuthenticated = errors.New(
	"the network is part of a Rune plan; run the `login` command to sign in")

// ErrSubscriptionRequired is returned when the signed-in account holds
// no plan that includes the network.
var ErrSubscriptionRequired = errors.New(
	"the network is part of a Rune plan; upgrade to join it")

// Peer is one other machine in the mesh.
type Peer struct {
	// Hostname is the short name used in rune://<hostname> URIs.
	Hostname string
	// DNSName is the peer's fully qualified mesh name.
	DNSName string
	// Addrs are the peer's mesh IP addresses.
	Addrs []netip.Addr
	// OS is the peer's operating system, as it reported it.
	OS string
	// Online is whether the peer is currently reachable.
	Online bool
}

// Status summarizes the local node's membership in the mesh.
type Status struct {
	Hostname string
	Addrs    []netip.Addr
	// ControlURL is the coordination server in use. Empty means the
	// Tailscale default.
	ControlURL string
	// State is the backend state: NotRunning, NeedsLogin, Starting,
	// Running, ...
	State string
	// AuthURL is set while the node waits for an interactive login.
	AuthURL string
	// LoginName identifies the account this node belongs to.
	LoginName string
	// LastError is the most recent failure to bring the node up or
	// join the mesh. It is empty once the node has joined.
	LastError string
}

// Node is the local machine's membership in the mesh.
type Node struct {
	cfg Config
	srv *tsnet.Server

	ctx       context.Context
	cancelCtx func()

	mu         sync.Mutex
	lc         *local.Client
	closed     bool
	started    bool
	loginName  string
	controlURL string
	// lastErr is the most recent failure to bring the node up or join
	// the mesh, kept so Status can report why the node is not Running
	// instead of the failure dying in a log line.
	lastErr error
}

// New builds a Node from cfg. The node stays offline until Start runs.
func New(cfg Config) *Node {
	logger := log.WithField(logging.KeyClass, "runenet")
	return &Node{
		cfg:        cfg,
		controlURL: cfg.ControlURL,
		srv: &tsnet.Server{
			Dir:        filepath.Join(cfg.Dir, "runenet"),
			Hostname:   cfg.Hostname,
			AuthKey:    cfg.AuthKey,
			ControlURL: cfg.ControlURL,
			UserLogf:   logger.Infof,
			Logf:       logger.Tracef,
		},
	}
}

// Start brings the node online. It returns as soon as the backend is
// running locally; joining the mesh (which may need an interactive
// login) continues in the background, so callers that must observe a
// joined node call [Node.Up] or poll [Node.Status] instead of blocking
// here. Every failure, including the background join's, is recorded
// and reported by [Node.Status].
//
// ctx bounds the credentials round-trip only; the join itself outlives
// it.
func (n *Node) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return ErrNotStarted
	}
	if n.lc != nil {
		return nil
	}
	if n.cfg.Credentials != nil {
		controlURL, authKey, err := n.cfg.Credentials(ctx)
		if err != nil {
			n.lastErr = err
			return err
		}
		n.srv.ControlURL = controlURL
		n.srv.AuthKey = authKey
		n.controlURL = controlURL
	}
	// The join outlives whatever call started the node, so its context
	// is the node's own lifetime and nothing else.
	n.ctx, n.cancelCtx = context.WithCancel(context.Background())
	if err := n.srv.Start(); err != nil {
		n.cancelCtx()
		n.lastErr = fmt.Errorf("start network node: %w", err)
		return n.lastErr
	}
	n.started = true
	lc, err := n.srv.LocalClient()
	if err != nil {
		n.cancelCtx()
		_ = n.srv.Close()
		n.lastErr = fmt.Errorf("network node client: %w", err)
		return n.lastErr
	}
	n.lc = lc

	upCtx := n.ctx
	go debug.CapturePanicReport(func() {
		_, err := n.srv.Up(upCtx)
		n.recordJoin(err)
		if err != nil {
			log.WithField(logging.KeyClass, "runenet").
				Warnf("could not join network: %v", err)
		}
	})
	return nil
}

// recordJoin stores the outcome of a join attempt for Status to
// report; success clears earlier failures. Attempts cut short by Close
// are discarded: "context canceled" explains nothing about the mesh.
func (n *Node) recordJoin(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return
	}
	n.lastErr = err
}

// Up joins the mesh, starting the node first if needed, and blocks
// until this machine is a member or ctx ends. Unlike the background
// join that Start kicks off, it returns the join failure to the
// caller; the failure is also recorded for [Node.Status].
func (n *Node) Up(ctx context.Context) error {
	if err := n.Start(ctx); err != nil {
		return err
	}
	if err := n.setWantRunning(ctx, true); err != nil {
		return err
	}
	_, err := n.srv.Up(ctx)
	n.recordJoin(err)
	if err != nil {
		return fmt.Errorf("join network: %w", err)
	}
	return nil
}

// Down leaves the mesh but keeps this machine's identity, so a later
// Up rejoins without a new sign-in.
func (n *Node) Down(ctx context.Context) error {
	return n.setWantRunning(ctx, false)
}

func (n *Node) setWantRunning(ctx context.Context, running bool) error {
	lc, err := n.client()
	if err != nil {
		return err
	}
	_, err = lc.EditPrefs(ctx, &ipn.MaskedPrefs{
		Prefs:          ipn.Prefs{WantRunning: running},
		WantRunningSet: true,
	})
	if err != nil {
		return fmt.Errorf("set network state: %w", err)
	}
	return nil
}

// Close takes the node offline and releases its mesh resources.
func (n *Node) Close() error {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return nil
	}
	n.closed = true
	cancel := n.cancelCtx
	started := n.started
	n.lc = nil
	n.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	// tsnet panics when closing a server it never started, which is
	// the shape of a node that never got past the entitlement check.
	if !started {
		return nil
	}
	return n.srv.Close()
}

// client returns the local mesh control client, or ErrNotStarted.
func (n *Node) client() (*local.Client, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.lc == nil {
		return nil, ErrNotStarted
	}
	return n.lc, nil
}

// Listen accepts mesh connections on the configured port.
func (n *Node) Listen() (net.Listener, error) {
	if _, err := n.client(); err != nil {
		return nil, err
	}
	return n.srv.Listen("tcp", fmt.Sprintf(":%d", n.cfg.Port))
}

// Dial opens a connection to host's workspace port. host is either a
// peer name as reported by [Node.Peers] or a mesh IP address.
func (n *Node) Dial(ctx context.Context, host string) (net.Conn, error) {
	if _, err := n.client(); err != nil {
		return nil, err
	}
	addr, err := n.resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	return n.srv.Dial(ctx, "tcp",
		net.JoinHostPort(addr.String(), fmt.Sprint(n.cfg.Port)))
}

// resolve maps a peer name to a mesh address. Names are resolved
// against the netmap rather than DNS so dialing does not depend on the
// host's resolver configuration.
func (n *Node) resolve(ctx context.Context, host string) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr, nil
	}
	peers, err := n.Peers(ctx)
	if err != nil {
		return netip.Addr{}, err
	}
	for _, p := range peers {
		if !strings.EqualFold(p.Hostname, host) &&
			!strings.EqualFold(trimDNSName(p.DNSName), host) {
			continue
		}
		if len(p.Addrs) == 0 {
			return netip.Addr{}, fmt.Errorf("peer %q has no address", host)
		}
		return p.Addrs[0], nil
	}
	return netip.Addr{}, fmt.Errorf("%q: %w", host, ErrPeerNotFound)
}

// Peers lists the other machines in the mesh, sorted by name.
func (n *Node) Peers(ctx context.Context) ([]Peer, error) {
	st, err := n.status(ctx)
	if err != nil {
		return nil, err
	}
	ret := make([]Peer, 0, len(st.Peer))
	for _, p := range st.Peer {
		ret = append(ret, Peer{
			Hostname: peerName(p),
			DNSName:  strings.TrimSuffix(p.DNSName, "."),
			Addrs:    p.TailscaleIPs,
			OS:       p.OS,
			Online:   p.Online,
		})
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Hostname < ret[j].Hostname
	})
	return ret, nil
}

// Status reports the local node's membership in the mesh. It is
// best-effort by design — the console's `network status` calls it to
// find out why the network is down, so a node that never came up
// reports state NotRunning and the recorded failure instead of an
// error.
func (n *Node) Status(ctx context.Context) (Status, error) {
	n.mu.Lock()
	lc := n.lc
	controlURL := n.controlURL
	lastErr := n.lastErr
	n.mu.Unlock()

	ret := Status{
		Hostname:   n.cfg.Hostname,
		ControlURL: controlURL,
	}
	if lastErr != nil {
		ret.LastError = lastErr.Error()
	}
	if lc == nil {
		ret.State = "NotRunning"
		return ret, nil
	}
	st, err := lc.Status(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("network status: %w", err)
	}
	ret.Addrs = st.TailscaleIPs
	ret.State = st.BackendState
	ret.AuthURL = st.AuthURL
	ret.LoginName = selfLoginName(st)
	return ret, nil
}

func (n *Node) status(ctx context.Context) (*ipnstate.Status, error) {
	lc, err := n.client()
	if err != nil {
		return nil, err
	}
	st, err := lc.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("network status: %w", err)
	}
	return st, nil
}

// selfLoginName is the account that owns the local node. It is empty
// until the node has logged in.
func selfLoginName(st *ipnstate.Status) string {
	if st.Self == nil {
		return ""
	}
	return st.User[st.Self.UserID].LoginName
}

// peerName prefers the mesh DNS label over HostInfo's hostname: the
// former is what the coordination server made unique across the
// tailnet, and therefore what rune:// URIs must use.
func peerName(p *ipnstate.PeerStatus) string {
	if name := trimDNSName(p.DNSName); name != "" {
		return name
	}
	return p.HostName
}

// trimDNSName reduces "host.tailnet.ts.net." to "host".
func trimDNSName(dnsName string) string {
	name, _, _ := strings.Cut(strings.TrimSuffix(dnsName, "."), ".")
	return name
}
