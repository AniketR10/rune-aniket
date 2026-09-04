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

package workspacessh

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	gitknownhosts "github.com/go-git/go-git/v6/plumbing/transport/ssh/knownhosts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"golang.org/x/crypto/ssh"
)

// anyKeyServer starts an in-process ssh server that accepts every public
// key, so host-key tests can focus on host-key verification rather than
// user authentication.
func anyKeyServer(t *testing.T) *inProcessServer {
	t.Helper()
	return startInProcessServer(t, &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	})
}

// hostKeyTestConfig returns a config pointing at srv with the given
// known_hosts path and strict-checking setting. A client key is always
// configured so auth succeeds without prompting.
func hostKeyTestConfig(t *testing.T, khPath string, strict bool, keyPath string) sshConfig {
	t.Helper()
	return sshConfig{
		privateKeys:           []string{keyPath},
		timeout:               2 * time.Second,
		knownHostsPath:        khPath,
		strictHostKeyChecking: strict,
	}
}

// randomPubKey returns a fresh, valid ed25519 ssh.PublicKey unrelated to
// any server, used to pin a "wrong" host key for changed-key scenarios.
func randomPubKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	k, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return k
}

func newRSAServerSigner(t *testing.T) ssh.Signer {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	return signer
}

// khEntryFor formats a plain known_hosts line for host:port and key.
func khEntryFor(host, port string, key ssh.PublicKey) string {
	addr := host
	if port != "22" {
		addr = "[" + host + "]:" + port
	}
	return addr + " " + string(ssh.MarshalAuthorizedKey(key))
}

// hashedKnownHostsLine builds an OpenSSH hashed known_hosts entry (|1|...)
// for the already-normalized host token and key.
func hashedKnownHostsLine(t *testing.T, normalizedHost string, key ssh.PublicKey) string {
	t.Helper()
	salt := make([]byte, sha1.Size)
	_, err := rand.Read(salt)
	require.NoError(t, err)
	mac := hmac.New(sha1.New, salt)
	mac.Write([]byte(normalizedHost))
	pattern := "|1|" + base64.StdEncoding.EncodeToString(salt) + "|" +
		base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return pattern + " " + string(ssh.MarshalAuthorizedKey(key))
}

// keyRecordedFor reports whether khPath (parsed via NewDB) pins key for
// host:port.
func keyRecordedFor(t *testing.T, khPath, hostport string, key ssh.PublicKey) bool {
	t.Helper()
	db, err := gitknownhosts.NewDB(khPath)
	require.NoError(t, err)
	for _, k := range db.HostKeys(hostport) {
		if string(k.Marshal()) == string(key.Marshal()) {
			return true
		}
	}
	return false
}

func TestHostKeyUnknownAcceptRecordsAndConnects(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)

	// Empty known_hosts: the host is unknown.
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, nil, 0o600))

	ui := &recordingUI{choices: []int{0}} // accept
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err, "unknown host + accept must connect")
	_ = r.Close()

	require.Len(t, ui.prompts, 1, "exactly one prompt expected; saw %v", ui.prompts)
	assert.Contains(t, ui.prompts[0], "choice:")
	assert.Contains(t, ui.prompts[0], "Unknown host")

	assert.True(t,
		keyRecordedFor(t, khPath, net.JoinHostPort(host, port), srv.hostKey.PublicKey()),
		"accepted host key must be persisted to known_hosts")
}

func TestHostKeyUnknownCancelLeavesFileUnchanged(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, nil, 0o600))

	ui := &recordingUI{choiceCancel: true}
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	_, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrHostKeyUnknown),
		"cancelling an unknown host must surface ErrHostKeyUnknown; got %v", err)

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Empty(t, b, "cancelled prompt must not write to known_hosts")
}

func TestHostKeyChangedAcceptReplacesStalePreservesOthers(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)
	hostport := net.JoinHostPort(host, port)

	// Pin a wrong key for our host, plus an unrelated host entry that
	// must survive the rewrite.
	wrong := randomPubKey(t)
	unrelated := randomPubKey(t)
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	content := khEntryFor(host, port, wrong) +
		khEntryFor("other.example.com", "22", unrelated)
	require.NoError(t, os.WriteFile(khPath, []byte(content), 0o600))

	ui := &recordingUI{choices: []int{0}} // accept new key
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err, "changed key + accept must connect")
	_ = r.Close()

	require.Len(t, ui.prompts, 1, "exactly one prompt expected; saw %v", ui.prompts)
	assert.Contains(t, ui.prompts[0], "Host key changed")

	assert.True(t,
		keyRecordedFor(t, khPath, hostport, srv.hostKey.PublicKey()),
		"new host key must be recorded")
	assert.False(t,
		keyRecordedFor(t, khPath, hostport, wrong),
		"stale host key must be removed")
	assert.True(t,
		keyRecordedFor(t, khPath, "other.example.com:22", unrelated),
		"unrelated host entry must be preserved")
}

func TestHostKeyChangedCancelLeavesFileUnchanged(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)

	wrong := randomPubKey(t)
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	content := khEntryFor(host, port, wrong)
	require.NoError(t, os.WriteFile(khPath, []byte(content), 0o600))

	ui := &recordingUI{choiceCancel: true}
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	_, err = newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrHostKeyMismatch),
		"cancelling a changed key must surface ErrHostKeyMismatch; got %v", err)

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Equal(t, content, string(b),
		"cancelled prompt must not modify known_hosts")
}

func TestHostKeyUnknownTrustOnceConnectsWithoutRecording(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, nil, 0o600))

	ui := &recordingUI{choices: []int{1}} // trust once
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err, "unknown host + trust once must connect")
	_ = r.Close()

	require.Len(t, ui.prompts, 1, "exactly one prompt expected; saw %v", ui.prompts)

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Empty(t, b,
		"trust once must connect without writing to known_hosts")
}

func TestHostKeyChangedTrustOnceConnectsWithoutRewriting(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)

	wrong := randomPubKey(t)
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	content := khEntryFor(host, port, wrong)
	require.NoError(t, os.WriteFile(khPath, []byte(content), 0o600))

	ui := &recordingUI{choices: []int{1}} // trust once
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err, "changed key + trust once must connect")
	_ = r.Close()

	require.Len(t, ui.prompts, 1, "exactly one prompt expected; saw %v", ui.prompts)

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Equal(t, content, string(b),
		"trust once must leave the stale known_hosts entry unchanged")
}

func TestSessionHostKeyPinUsesExactHostPort(t *testing.T) {
	key := randomPubKey(t)
	pin := new(sessionHostKeyPin)
	pin.put("host.example.com:22", key)

	got, ok := pin.get("host.example.com:22")
	require.True(t, ok)
	assert.Equal(t, key.Marshal(), got.Marshal())
	_, ok = pin.get("host.example.com:2222")
	assert.False(t, ok)

	var nilPin *sessionHostKeyPin
	_, ok = nilPin.get("host.example.com:22")
	assert.False(t, ok)
	assert.NotPanics(t, func() { nilPin.put("host.example.com:22", key) })
}

func TestHostKeyChangedTrustOnceReusesSessionPin(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	stale := []byte(khEntryFor(host, port, randomPubKey(t)))
	require.NoError(t, os.WriteFile(khPath, stale, 0o600))

	ui := &recordingUI{choices: []int{1}}
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")
	pin := new(sessionHostKeyPin)

	for range 2 {
		r, dialErr := newStdRemote(context.Background(), cfg, uri, ui, nil, pin)
		require.NoError(t, dialErr)
		_ = r.Close()
	}

	assert.Len(t, ui.prompts, 1)
	got, err := os.ReadFile(khPath)
	require.NoError(t, err)
	assert.Equal(t, stale, got)
}

func TestHostKeyChangedTrustOnceFreshSessionPromptsAgain(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	stale := []byte(khEntryFor(host, port, randomPubKey(t)))
	require.NoError(t, os.WriteFile(khPath, stale, 0o600))
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	firstUI := &recordingUI{choices: []int{1}}
	r, err := newStdRemote(context.Background(), cfg, uri, firstUI, nil, new(sessionHostKeyPin))
	require.NoError(t, err)
	_ = r.Close()

	secondUI := &recordingUI{choiceCancel: true}
	_, err = newStdRemote(context.Background(), cfg, uri, secondUI, nil, new(sessionHostKeyPin))
	require.ErrorIs(t, err, ErrHostKeyMismatch)
	assert.Len(t, secondUI.prompts, 1)
	got, err := os.ReadFile(khPath)
	require.NoError(t, err)
	assert.Equal(t, stale, got)
}

func TestHostKeyTrustOnceEntryPathsReuseSessionPin(t *testing.T) {
	tests := []struct {
		name       string
		knownHosts []byte
		choice     int
		strict     bool
		prompts    int
	}{
		{name: "unknown host", knownHosts: []byte{}, choice: 1, strict: true, prompts: 1},
		{name: "unparsable known hosts", knownHosts: []byte(malformedKnownHosts), strict: true, prompts: 1},
		{name: "unparsable strict disabled", knownHosts: []byte(malformedKnownHosts)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := anyKeyServer(t)
			keyPath, _ := generateClientKey(t)
			khPath := filepath.Join(t.TempDir(), "known_hosts")
			require.NoError(t, os.WriteFile(khPath, tt.knownHosts, 0o600))
			cfg := hostKeyTestConfig(t, khPath, tt.strict, keyPath)
			uri := uriForServer(t, srv, "")
			ui := &recordingUI{choices: []int{tt.choice}}
			pin := new(sessionHostKeyPin)

			for range 2 {
				r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, pin)
				require.NoError(t, err)
				_ = r.Close()
			}

			assert.Len(t, ui.prompts, tt.prompts)
			got, err := os.ReadFile(khPath)
			require.NoError(t, err)
			assert.Equal(t, tt.knownHosts, got)
		})
	}
}

func TestSessionHostKeyCallbackRejectsDifferentExactKey(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	knownHosts := []byte("# session pin must bypass this file\n")
	require.NoError(t, os.WriteFile(khPath, knownHosts, 0o600))
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")
	pinned := randomPubKey(t)
	pin := new(sessionHostKeyPin)
	pin.put(hostPortFromURI(uri), pinned)
	ui := &recordingUI{choiceCancel: true}

	_, err := newStdRemote(context.Background(), cfg, uri, ui, nil, pin)
	require.ErrorIs(t, err, ErrHostKeyMismatch)
	assert.Empty(t, ui.prompts)
	gotPin, ok := pin.get(hostPortFromURI(uri))
	require.True(t, ok)
	assert.Equal(t, pinned.Marshal(), gotPin.Marshal())
	gotKnownHosts, err := os.ReadFile(khPath)
	require.NoError(t, err)
	assert.Equal(t, knownHosts, gotKnownHosts)
}

func TestSessionHostKeyRSAAlgorithms(t *testing.T) {
	pinned := newRSAServerSigner(t).PublicKey()
	_, algorithms, err := buildHostkeyCallback(sshConfig{}, workspaceapi.URI{}, pinned)
	require.NoError(t, err)

	assert.Equal(t, []string{
		ssh.KeyAlgoRSASHA512,
		ssh.KeyAlgoRSASHA256,
		ssh.KeyAlgoRSA,
	}, algorithms)
}

func TestSessionHostKeyNegotiationFailureIsMismatch(t *testing.T) {
	serverConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	srv := startInProcessServerWithHostKey(t, serverConfig, newRSAServerSigner(t))
	keyPath, _ := generateClientKey(t)
	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, nil, 0o600))
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")
	pin := new(sessionHostKeyPin)
	pin.put(hostPortFromURI(uri), randomPubKey(t))
	ui := &recordingUI{choiceCancel: true}

	_, err := newStdRemote(context.Background(), cfg, uri, ui, nil, pin)
	require.ErrorIs(t, err, ErrHostKeyMismatch)
	assert.Empty(t, ui.prompts)
}

func TestHostKeyMatchingDoesNotPrompt(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)

	// configForCallbackTest already pins the server's real host key.
	cfg := configForCallbackTest(t, srv, []string{keyPath})
	cfg.strictHostKeyChecking = true
	uri := uriForServer(t, srv, "")

	ui := &recordingUI{choiceCancel: true} // would fail if prompted
	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err)
	_ = r.Close()
	assert.Empty(t, ui.prompts,
		"a matching known_hosts entry must never prompt; saw %v", ui.prompts)
}

func TestHostKeyStrictDisabledRecordsWithoutPrompt(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)
	host, port, err := net.SplitHostPort(srv.addr())
	require.NoError(t, err)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, nil, 0o600))

	ui := &recordingUI{choiceCancel: true} // must not be consulted
	cfg := hostKeyTestConfig(t, khPath, false, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err,
		"strict_host_key_checking=false must auto-accept and connect")
	_ = r.Close()
	assert.Empty(t, ui.prompts,
		"strict=false must not prompt; saw %v", ui.prompts)
	assert.True(t,
		keyRecordedFor(t, khPath, net.JoinHostPort(host, port), srv.hostKey.PublicKey()),
		"strict=false must still record the accepted key")
}

// malformedKnownHosts is a known_hosts line that exists but cannot be
// parsed (invalid base64 key), used to exercise the unparsable path.
const malformedKnownHosts = "host.example.com ssh-ed25519 not_valid_base64!!!\n"

func TestHostKeyUnparsableTrustOnceConnectsWithoutRewriting(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, []byte(malformedKnownHosts), 0o600))

	ui := &recordingUI{choices: []int{0}} // trust once (only accept option)
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err, "unparsable known_hosts + trust once must connect")
	_ = r.Close()

	require.Len(t, ui.prompts, 1, "exactly one prompt expected; saw %v", ui.prompts)
	assert.Contains(t, ui.prompts[0], "Can't read known_hosts")

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Equal(t, malformedKnownHosts, string(b),
		"a malformed known_hosts must never be rewritten")
}

func TestHostKeyUnparsableCancelSurfacesError(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, []byte(malformedKnownHosts), 0o600))

	ui := &recordingUI{choiceCancel: true}
	cfg := hostKeyTestConfig(t, khPath, true, keyPath)
	uri := uriForServer(t, srv, "")

	_, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrKnownHostsUnparsable),
		"cancelling an unparsable known_hosts must surface "+
			"ErrKnownHostsUnparsable; got %v", err)

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Equal(t, malformedKnownHosts, string(b),
		"cancelled prompt must not modify known_hosts")
}

func TestHostKeyUnparsableStrictDisabledTrustsOnceWithoutPrompt(t *testing.T) {
	srv := anyKeyServer(t)
	keyPath, _ := generateClientKey(t)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(khPath, []byte(malformedKnownHosts), 0o600))

	ui := &recordingUI{choiceCancel: true} // must not be consulted
	cfg := hostKeyTestConfig(t, khPath, false, keyPath)
	uri := uriForServer(t, srv, "")

	r, err := newStdRemote(context.Background(), cfg, uri, ui, nil, nil)
	require.NoError(t, err,
		"strict=false must trust once and connect despite unparsable file")
	_ = r.Close()
	assert.Empty(t, ui.prompts,
		"strict=false must not prompt; saw %v", ui.prompts)

	b, readErr := os.ReadFile(khPath)
	require.NoError(t, readErr)
	assert.Equal(t, malformedKnownHosts, string(b),
		"strict=false must not rewrite a malformed known_hosts")
}

func TestBuildHostkeyCallbackPinsAlgorithms(t *testing.T) {
	srv := anyKeyServer(t)
	cfg := configForCallbackTest(t, srv, nil)
	uri := uriForServer(t, srv, "")

	_, algos, err := buildHostkeyCallback(cfg, uri, nil)
	require.NoError(t, err)
	require.NotEmpty(t, algos,
		"a known host must pin its host-key algorithms so negotiation "+
			"lands on a recorded key type")
	assert.Contains(t, algos, ssh.KeyAlgoED25519,
		"the in-process server uses an ed25519 host key")
}

func TestPersistKnownHostKey(t *testing.T) {
	remote := &net.TCPAddr{IP: net.IPv4(10, 0, 0, 6), Port: 2222}
	hostport := "10.0.0.6:2222"

	tests := []struct {
		name      string
		unknown   bool
		seed      func(t *testing.T, key ssh.PublicKey) string
		wantWrong bool // whether the pre-seeded wrong key should remain
	}{
		{
			name:    "append to existing file",
			unknown: true,
			seed: func(t *testing.T, _ ssh.PublicKey) string {
				p := filepath.Join(t.TempDir(), "known_hosts")
				require.NoError(t, os.WriteFile(p,
					[]byte(khEntryFor("keep.example.com", "22", randomPubKey(t))), 0o600))
				return p
			},
		},
		{
			name:    "create missing file and parent dir",
			unknown: true,
			seed: func(t *testing.T, _ ssh.PublicKey) string {
				return filepath.Join(t.TempDir(), "nested", "dir", "known_hosts")
			},
		},
		{
			name:    "replace stale line on change",
			unknown: false,
			seed: func(t *testing.T, _ ssh.PublicKey) string {
				p := filepath.Join(t.TempDir(), "known_hosts")
				require.NoError(t, os.WriteFile(p,
					[]byte(khEntryFor("10.0.0.6", "2222", randomPubKey(t))), 0o600))
				return p
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newKey := randomPubKey(t)
			khPath := tc.seed(t, newKey)
			e := &HostKeyError{
				Unknown:        tc.unknown,
				KnownHostsPath: khPath,
				Host:           "10.0.0.6",
				HostPort:       hostport,
				Remote:         remote,
				Presented:      newKey,
			}
			require.NoError(t, persistKnownHostKey(e))

			// Result must parse and pin the new key.
			db, err := gitknownhosts.NewDB(khPath)
			require.NoError(t, err)
			var found bool
			for _, k := range db.HostKeys(hostport) {
				if string(k.Marshal()) == string(newKey.Marshal()) {
					found = true
				}
			}
			assert.True(t, found, "new key must be recorded")

			info, err := os.Stat(khPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
				"known_hosts must be written 0600")
		})
	}
}

func TestPersistKnownHostKeyMatchesHashedEntry(t *testing.T) {
	// A hashed entry for the host must be recognized and removed on a
	// changed-key rewrite. We generate the hashed line with ssh-keygen-
	// compatible HMAC via the same helper the production code uses.
	remote := &net.TCPAddr{IP: net.IPv4(10, 0, 0, 6), Port: 2222}
	hostport := "10.0.0.6:2222"
	stale := randomPubKey(t)

	khPath := filepath.Join(t.TempDir(), "known_hosts")
	hashed := hashedKnownHostsLine(t, gitknownhosts.Normalize(hostport), stale)
	require.NoError(t, os.WriteFile(khPath, []byte(hashed), 0o600))

	newKey := randomPubKey(t)
	e := &HostKeyError{
		Unknown:        false,
		KnownHostsPath: khPath,
		Host:           "10.0.0.6",
		HostPort:       hostport,
		Remote:         remote,
		Presented:      newKey,
	}
	require.NoError(t, persistKnownHostKey(e))

	db, err := gitknownhosts.NewDB(khPath)
	require.NoError(t, err)
	var foundNew, foundStale bool
	for _, k := range db.HostKeys(hostport) {
		switch string(k.Marshal()) {
		case string(newKey.Marshal()):
			foundNew = true
		case string(stale.Marshal()):
			foundStale = true
		}
	}
	assert.True(t, foundNew, "new key must be recorded")
	assert.False(t, foundStale, "hashed stale entry must be removed")
}

func TestRemoveHostLinesPreservesCommentsAndBlanks(t *testing.T) {
	host := randomPubKey(t)
	other := randomPubKey(t)
	content := "# a comment\n\n" +
		khEntryFor("10.0.0.6", "2222", host) +
		khEntryFor("other", "22", other)
	got := removeHostLines([]byte(content), gitknownhosts.Normalize("10.0.0.6:2222"))
	gotStr := string(got)
	assert.Contains(t, gotStr, "# a comment")
	assert.Contains(t, gotStr, "other ")
	assert.NotContains(t, gotStr, "[10.0.0.6]:2222")
}

func TestTranslateDialErrorPassesThroughHostKeyError(t *testing.T) {
	unknown := &HostKeyError{
		Unknown: true,
		err:     errors.New("wrapped"),
	}
	got := translateDialError("h:22", false, false, unknown)
	var hk *HostKeyError
	require.True(t, errors.As(got, &hk))
	assert.Same(t, unknown, hk)
}
