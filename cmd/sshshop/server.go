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

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

type serverConfig struct {
	addr        string
	hostKeyPath string
	idleTimeout time.Duration
	maxTimeout  time.Duration
	banner      string
}

func newServer(cfg serverConfig, logger *slog.Logger) (*ssh.Server, error) {
	signer, err := loadOrCreateHostKey(cfg.hostKeyPath, logger)
	if err != nil {
		return nil, fmt.Errorf("host key: %w", err)
	}

	srv := &ssh.Server{
		Addr:        cfg.addr,
		Version:     "sshshop-0.1",
		Banner:      cfg.banner,
		HostSigners: []ssh.Signer{signer},
		IdleTimeout: cfg.idleTimeout,
		MaxTimeout:  cfg.maxTimeout,
		// PTY allowed for everyone. Any other callback is intentionally
		// left nil so reverse/local port forwarding is denied.
		PtyCallback: func(ssh.Context, ssh.Pty) bool { return true },
		// NoClientAuth: terminal.shop style. Identity happens inside
		// the TUI (email / magic-link / Stripe) rather than via SSH keys.
		PublicKeyHandler: nil,
		PasswordHandler:  nil,
		Handler: func(sess ssh.Session) {
			handleSession(sess, logger)
		},
	}
	return srv, nil
}

func handleSession(sess ssh.Session, logger *slog.Logger) {
	log := logger.With(
		"remote", sess.RemoteAddr().String(),
		"user", sess.User(),
	)

	// Reject exec ("ssh host cmd") and subsystem ("sftp") requests.
	// We only serve interactive shells.
	if len(sess.Command()) > 0 {
		_, _ = fmt.Fprintln(sess.Stderr(), "sshshop: exec is not supported")
		_ = sess.Exit(127)
		return
	}
	if sub := sess.Subsystem(); sub != "" {
		_, _ = fmt.Fprintf(sess.Stderr(), "sshshop: subsystem %q is not supported\n", sub)
		_ = sess.Exit(127)
		return
	}

	ctx := sess.Context()

	log.Info("session start")
	start := time.Now()
	if err := runSession(ctx, sess, log); err != nil && !errors.Is(err, ctx.Err()) {
		log.Warn("session error", "err", err)
	}
	log.Info("session end", "duration", time.Since(start))
}

// loadOrCreateHostKey loads an OpenSSH-format PEM ed25519 private key
// from path, generating a new one (and writing it) if the file does
// not exist. The resulting signer's SHA256 fingerprint is logged so
// you can pin it on the landing page.
func loadOrCreateHostKey(path string, logger *slog.Logger) (ssh.Signer, error) {
	pemBytes, err := os.ReadFile(path)
	switch {
	case err == nil:
		signer, err := gossh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("parse host key: %w", err)
		}
		logFingerprint(signer, logger, "loaded host key")
		return signer, nil

	case errors.Is(err, os.ErrNotExist):
		signer, pemBytes, err := generateEd25519HostKey()
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
			return nil, fmt.Errorf("write host key: %w", err)
		}
		logFingerprint(signer, logger, "generated host key")
		return signer, nil

	default:
		return nil, err
	}
}

func generateEd25519HostKey() (ssh.Signer, []byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	// OpenSSH marshalling so the file is also usable with ssh-keygen.
	pemBlock, err := gossh.MarshalPrivateKey(priv, "sshshop host key")
	if err != nil {
		return nil, nil, err
	}
	pemBytes := pem.EncodeToMemory(pemBlock)
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return signer, pemBytes, nil
}

func logFingerprint(signer ssh.Signer, logger *slog.Logger, msg string) {
	pub := signer.PublicKey()
	sum := sha256.Sum256(pub.Marshal())
	fp := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
	logger.Info(msg, "fingerprint", fp, "algo", pub.Type())
}
