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
