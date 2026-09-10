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

package workspacessh

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitknownhosts "github.com/go-git/go-git/v6/plumbing/transport/ssh/knownhosts"
	"golang.org/x/crypto/ssh"
)

const (
	cancelOption    = "    cancel    "
	trustAndConnect = "    trust     "
	trustOnce       = "  trust once  "
)

// hostKeyDecision is the outcome of a host-key trust prompt.
type hostKeyDecision int

const (
	// hostKeyReject aborts the connection.
	hostKeyReject hostKeyDecision = iota
	// hostKeyTrustPersist connects and records the key in known_hosts.
	hostKeyTrustPersist
	// hostKeyTrustOnce connects for this session only, without recording
	// the key in known_hosts.
	hostKeyTrustOnce
)

// promptTrustHostKey renders a markdown trust prompt for an unknown host,
// a changed host key, or an unparsable known_hosts file, and returns the
// user's decision. A cancelled prompt (context.Canceled) or the cancel
// option yields hostKeyReject.
//
// The unparsable case offers only trust-once and cancel: a malformed file
// can't be safely rewritten, so persisting is not an option.
func promptTrustHostKey(ctx context.Context, ui UI, e *HostKeyError) (hostKeyDecision, error) {
	if e.Unparsable {
		idx, err := ui.PromptChoice(ctx, unparsableHostPrompt(e),
			[]string{trustOnce, cancelOption})
		if err != nil {
			if err == context.Canceled {
				return hostKeyReject, nil
			}
			return hostKeyReject, err
		}
		if idx == 0 {
			return hostKeyTrustOnce, nil
		}
		return hostKeyReject, nil
	}

	var message string
	if e.Unknown {
		message = unknownHostPrompt(e)
	} else {
		message = changedHostKeyPrompt(e)
	}
	idx, err := ui.PromptChoice(ctx, message,
		[]string{trustAndConnect, trustOnce, cancelOption})
	if err != nil {
		if err == context.Canceled {
			return hostKeyReject, nil
		}
		return hostKeyReject, err
	}
	switch idx {
	case 0:
		return hostKeyTrustPersist, nil
	case 1:
		return hostKeyTrustOnce, nil
	default:
		return hostKeyReject, nil
	}
}

func unparsableHostPrompt(e *HostKeyError) string {
	return fmt.Sprintf(`## 󰀦  Can't read known_hosts

Rune could not parse `+"`%s`"+`, so the host key for **%s** can't be verified
against it.

- **Key type:** %s
- **Fingerprint:** `+"`SHA256:%s`"+`

Fix the file to record this host permanently. For now you can **trust once** to
connect for this session only; Rune will not change the file.`,
		e.KnownHostsPath,
		e.Host,
		e.Presented.Type(),
		sha256Fingerprint(e.Presented),
	)
}

func unknownHostPrompt(e *HostKeyError) string {
	return fmt.Sprintf(`## 󰌆  Unknown host

The authenticity of **%s** can't be established. This is the first time
Rune is connecting to it.

- **Key type:** %s
- **Fingerprint:** `+"`SHA256:%s`"+`

If you recognize this host, trust its key to continue. Rune will remember it in
`+"`%s`"+`. Choose **trust once** to connect for this session only without
recording the key.`,
		e.Host,
		e.Presented.Type(),
		sha256Fingerprint(e.Presented),
		e.KnownHostsPath,
	)
}

func changedHostKeyPrompt(e *HostKeyError) string {
	var previously strings.Builder
	for _, k := range e.KnownKeys {
		fmt.Fprintf(&previously, "- **Previously trusted:** `SHA256:%s`\n",
			sha256Fingerprint(k))
	}
	return fmt.Sprintf(`## 󰌊  Host key changed for %s

The host key for **%s** does **not** match the key recorded in
`+"`%s`"+`.

%s- **Now presented:** `+"`SHA256:%s`"+`
- **Key type:** %s

This can happen if the server was reinstalled or rekeyed — **or** it can
indicate a man-in-the-middle attack. Only continue if you expected this change.
Trusting will replace the old key in `+"`known_hosts`"+`; choose **trust once**
to connect for this session only without changing `+"`known_hosts`"+`.`,
		e.Host,
		e.Host,
		e.KnownHostsPath,
		previously.String(),
		sha256Fingerprint(e.Presented),
		e.Presented.Type(),
	)
}

// sha256Fingerprint returns the fingerprint without the leading "SHA256:"
// prefix that ssh.FingerprintSHA256 adds, so callers control the label.
func sha256Fingerprint(key ssh.PublicKey) string {
	return strings.TrimPrefix(ssh.FingerprintSHA256(key), "SHA256:")
}

// persistKnownHostKey records the presented host key in the known_hosts
// file. For a changed key it first removes the stale line(s) for the host,
// then appends the new entry. The file is created (0600) with its parent
// directory (0700) if missing, and written atomically.
func persistKnownHostKey(e *HostKeyError) error {
	path := e.KnownHostsPath
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var out bytes.Buffer
	if len(existing) > 0 {
		kept := existing
		if !e.Unknown {
			kept = removeHostLines(existing, e.HostPort)
		}
		out.Write(kept)
		if out.Len() > 0 && out.Bytes()[out.Len()-1] != '\n' {
			out.WriteByte('\n')
		}
	}

	if err := gitknownhosts.WriteKnownHost(&out, e.HostPort, e.Remote, e.Presented); err != nil {
		return err
	}

	return writeFileAtomic(path, out.Bytes(), 0o600)
}

// removeHostLines drops every known_hosts line whose host pattern matches
// the given host:port, preserving all other lines (including comments and
// blank lines) in order. Both plain and hashed host patterns are handled
// via knownhosts.Normalize equality / x/crypto hashed matching.
func removeHostLines(content []byte, hostport string) []byte {
	target := gitknownhosts.Normalize(hostport)
	var out bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if hostLineMatches(line, target) {
			continue
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// hostLineMatches reports whether a single known_hosts line applies to the
// given host. Comment/blank lines never match. The host-pattern field (the
// first token, possibly comma-separated and possibly hashed) is compared
// against the normalized host:port.
func hostLineMatches(line []byte, normalized string) bool {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 || trimmed[0] == '#' {
		return false
	}
	// Skip an optional @marker (e.g. @cert-authority, @revoked); we never
	// remove CA lines here, only plain host entries.
	fields := bytes.Fields(trimmed)
	patternField := fields[0]
	if bytes.HasPrefix(patternField, []byte("@")) {
		return false
	}
	for _, pat := range bytes.Split(patternField, []byte(",")) {
		if hostPatternMatches(pat, normalized) {
			return true
		}
	}
	return false
}

// hostPatternMatches compares a single host pattern token against the
// target host. Hashed patterns (|1|...) are matched by re-hashing the
// target with the embedded salt; plain patterns by normalized equality.
func hostPatternMatches(pattern []byte, normalized string) bool {
	if bytes.HasPrefix(pattern, []byte("|1|")) {
		return hashedHostMatches(string(pattern), normalized)
	}
	return gitknownhosts.Normalize(string(pattern)) == normalized
}

// hashedHostMatches reports whether a hashed known_hosts host pattern of the
// form "|1|<b64-salt>|<b64-hmac>" matches the target host. OpenSSH hashes
// the normalized host token (HMAC-SHA1 keyed by the salt), so we recompute
// the HMAC of the normalized host and compare.
func hashedHostMatches(pattern, normalized string) bool {
	parts := strings.Split(pattern, "|")
	// parts: ["", "1", "<salt>", "<hash>"]
	if len(parts) != 4 {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	mac := hmac.New(sha1.New, salt)
	mac.Write([]byte(normalized))
	return hmac.Equal(mac.Sum(nil), want)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".known_hosts-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
