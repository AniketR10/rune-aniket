// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package workspacessh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/unstablebuild/blue/bluectx"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace"
)

const watcherWaitTimeout = 2 * time.Minute

// ErrAuthRequiredKey indicates the server requires publickey authentication
// but the client has no usable signers configured.
var ErrAuthRequiredKey = errors.New(
	"server requires publickey authentication but no client keys are configured. " +
		"Add a path to ssh.private_keys (or store an existing key under ~/.ssh)")

// ErrHostKeyMismatch indicates the server's host key did not match the entry
// recorded in known_hosts. Surfaced as a hard error: never prompts the user,
// because this typically indicates a man-in-the-middle attack.
var ErrHostKeyMismatch = errors.New(
	"host key verification failed: the server's host key does not match the " +
		"entry recorded in known_hosts")

// ErrHostUnreachable wraps low-level network failures (DNS, TCP).
var ErrHostUnreachable = errors.New("could not reach ssh host")

var sigMap = map[syscall.Signal]ssh.Signal{
	syscall.SIGABRT: "ABRT",
	syscall.SIGALRM: "ALRM",
	syscall.SIGFPE:  "FPE",
	syscall.SIGHUP:  "HUP",
	syscall.SIGILL:  "ILL",
	syscall.SIGINT:  "INT",
	syscall.SIGKILL: "KILL",
	syscall.SIGPIPE: "PIPE",
	syscall.SIGQUIT: "QUIT",
	syscall.SIGSEGV: "SEGV",
	syscall.SIGTERM: "TERM",
	syscall.SIGUSR1: "USR1",
	syscall.SIGUSR2: "USR2",
}

// used to adapt ssh.Client to sshClient
type stdRemote struct {
	parentCtx context.Context
	client    *ssh.Client
	quitCh    chan struct{}
}

// used to adapt ssh.Session to Executor
type goSshSession struct {
	parentCtx context.Context
	ses       *ssh.Session
	quitCh    chan struct{}
	pid       int
}

func newStdRemote(
	ctx context.Context, cfg sshConfig, uri workspaceapi.URI, ui UI,
) (remote, error) {
	username, err := usernameOrCurrent(uri)
	if err != nil {
		return nil, err
	}
	auths, err := authMethodsFromURI(ctx, cfg, uri, ui)
	if err != nil {
		return nil, err
	}

	var hostkeyCallback ssh.HostKeyCallback
	if cfg.insecure {
		hostkeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		hostkeyCallback, err = defaultHostkeyCallback(cfg.knownHostsPath)
		if err != nil {
			return nil, err
		}
	}
	conf := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: hostkeyCallback,
		Auth:            auths,
		Timeout:         cfg.timeout,
	}

	hostport := hostPortFromURI(uri)
	conn, err := ssh.Dial("tcp", hostport, conf)
	if err != nil {
		return nil, translateDialError(hostport, len(cfg.privateKeys) > 0, err)
	}
	ret := &stdRemote{parentCtx: ctx, client: conn, quitCh: make(chan struct{})}
	return ret, nil
}

func (s *goSshSession) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	if s.pid != 0 {
		panic("Command called more than once on an ssh session")
	}
	if cmd.Path == "" {
		return 0, errors.New("no command")
	}

	s.pid++

	s.ses.Stdout = cmd.Stdout
	s.ses.Stderr = cmd.Stderr
	s.ses.Stdin = cmd.Stdin

	err := s.ses.Start(
		fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args, " ")))
	if err != nil {
		return 0, err
	}

	// ensure that at least one of the ctxs passed to First
	// is canceled after the command is done.
	ctx, cancel := bluectx.First(s.parentCtx, ctx)

	// wait and dispatch error to watcher
	go debug.CapturePanicReport(func() {
		defer cancel()

		err := s.ses.Wait()
		if cmd.Watcher != nil && cmd.Watcher.WatchProcess() != nil {
			// avoid buggy watchers to cause this goroutine to block forever,
			// so the timeout should be in the order of minutes.
			ctx, cancelTimeout := context.WithTimeout(
				context.Background(), watcherWaitTimeout)
			defer cancelTimeout()
			select {
			case <-ctx.Done():
			case cmd.Watcher.WatchProcess() <- err:
			}
		}
	})

	// kill command if context is done
	go debug.CapturePanicReport(func() {
		select {
		case <-ctx.Done():
			s.ses.Close()
		case <-s.quitCh:
		}
	})

	return workspaceapi.Pid(s.pid), nil
}

func (s *goSshSession) Signal(_ workspaceapi.Pid, sig syscall.Signal) error {
	signal, ok := sigMap[sig]
	if !ok {
		return errors.New("unknown signal")
	}
	return s.ses.Signal(signal)
}

func (r *goSshSession) Close() error {
	return r.ses.Close()
}

func (r *stdRemote) NewSession() (schemeapi.Executor, error) {
	ses, err := r.client.NewSession()
	if err != nil {
		return nil, err
	}
	ret := &goSshSession{parentCtx: r.parentCtx, ses: ses, quitCh: r.quitCh}
	return ret, nil
}

func (r *stdRemote) Close() error {
	close(r.quitCh)
	return r.client.Close()
}

// authMethodsFromURI builds the list of ssh.AuthMethod values offered to
// the server. Methods are arranged so that passive credentials (URI
// password, configured private keys) are tried first, with interactive
// callbacks (password, keyboard-interactive) falling back to ui prompts.
//
// The Go ssh client only invokes a callback when the server's
// methodsAllowed list advertises the corresponding method, so the user
// only sees prompts that can succeed.
func authMethodsFromURI(
	ctx context.Context, cfg sshConfig, uri workspaceapi.URI, ui UI,
) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod

	// 1. URI-embedded password takes precedence: no prompt needed.
	if pass, ok := uri.Password(); ok {
		auths = append(auths, ssh.Password(pass))
	}

	// 2. Configured private keys (with on-demand passphrase prompts).
	auths = append(auths, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		return gatherSigners(ctx, cfg, ui)
	}))

	// 3. Interactive password fallback, retried up to 3 times. We track
	//    whether the previous attempt succeeded by observing two
	//    consecutive callbacks: PasswordCallback is only re-invoked by
	//    ssh.RetryableAuthMethod when the prior credential was rejected,
	//    so on the 2nd+ call we can confidently surface a notification
	//    explaining why the user is being prompted again.
	var passAttempts int
	auths = append(auths, ssh.RetryableAuthMethod(
		ssh.PasswordCallback(func() (string, error) {
			if passAttempts > 0 {
				ui.Notify(NotificationError, fmt.Sprintf(
					"ssh: password rejected (attempt %d). Try again or press esc to cancel.",
					passAttempts))
			}
			passAttempts++
			return ui.PromptSecret(ctx, "ssh password: ")
		}),
		3,
	))

	// 4. Optional keyboard-interactive (PAM-style) challenges, retried up
	//    to 3 times. NOTE: the Go ssh client unconditionally invokes this
	//    AuthMethod when the server's USERAUTH_FAILURE reply lists
	//    "keyboard-interactive"; OpenSSH advertises it even when no
	//    challenge is configured, which surfaces as "unexpected message
	//    type 51 (expected 60)". Off by default; opt in via the
	//    `kbd_interactive` config key.
	if cfg.kbdInteractive {
		auths = append(auths, ssh.RetryableAuthMethod(
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				return askKbd(ctx, ui, name, instruction, questions, echos)
			}),
			3,
		))
	}

	return auths, nil
}

// gatherSigners parses configured private key paths into ssh.Signer values.
// When a key file is encrypted, ui.PromptSecret is used to collect the
// passphrase. Keys that fail to parse are notified to the user but skipped,
// so a single broken key does not prevent the others from being tried.
func gatherSigners(ctx context.Context, cfg sshConfig, ui UI) ([]ssh.Signer, error) {
	keyPaths := cfg.privateKeys
	if len(keyPaths) == 0 {
		// Mirror OpenSSH behaviour: if no keys are configured, try the
		// canonical default identity files under ~/.ssh/. This is a
		// common, low-risk strategy (the same one OpenSSH uses by
		// default) and avoids forcing users to spell out paths in
		// workspace config when their keys are in the standard location.
		keyPaths = defaultIdentityFiles()
	}
	var signers []ssh.Signer
	for _, keyPath := range keyPaths {
		signer, err := signerForKey(ctx, keyPath, ui)
		if err != nil {
			ui.Notify(NotificationWarning,
				fmt.Sprintf("ssh: skipping key %q: %v", keyPath, err))
			continue
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

// defaultIdentityFiles returns the list of canonical SSH identity files
// under the current user's ~/.ssh/ directory that exist on disk. The
// order mirrors OpenSSH's default IdentityFile preference.
func defaultIdentityFiles() []string {
	// os.UserHomeDir honours the HOME env var, which makes this function
	// straightforward to drive from tests without having to mock the
	// user database.
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		"id_ed25519",
		"id_ecdsa",
		"id_ecdsa_sk",
		"id_ed25519_sk",
		"id_rsa",
		"id_dsa",
	}
	var found []string
	for _, name := range candidates {
		p := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// signerForKey reads and parses a single private key file, prompting for a
// passphrase via ui when needed.
func signerForKey(ctx context.Context, keyPath string, ui UI) (ssh.Signer, error) {
	privateKey, err := workspace.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read private key file: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err == nil {
		return signer, nil
	}
	if _, ok := err.(*ssh.PassphraseMissingError); !ok {
		return nil, fmt.Errorf("could not parse private key: %w", err)
	}
	passphrase, err := ui.PromptSecret(ctx,
		fmt.Sprintf("passphrase for ssh key %s: ", keyPath))
	if err != nil {
		return nil, fmt.Errorf("passphrase prompt: %w", err)
	}
	signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKey, []byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf("could not parse private key with passphrase: %w", err)
	}
	return signer, nil
}

// askKbd answers a single round of keyboard-interactive challenges. Each
// non-empty question is forwarded to ui.PromptSecret (echo=false) or
// ui.PromptText (echo=true).
func askKbd(
	ctx context.Context, ui UI,
	name, instruction string, questions []string, echos []bool,
) ([]string, error) {
	answers := make([]string, len(questions))
	for i, q := range questions {
		label := q
		if instruction != "" && i == 0 {
			label = instruction + "\n" + q
		}
		_ = name
		var (
			ans string
			err error
		)
		if i < len(echos) && echos[i] {
			ans, err = ui.PromptText(ctx, label, "")
		} else {
			ans, err = ui.PromptSecret(ctx, label)
		}
		if err != nil {
			return nil, err
		}
		answers[i] = ans
	}
	return answers, nil
}

// translateDialError maps a raw ssh.Dial error to a user-friendly typed
// error. The Go ssh library wraps server-rejection failures in errors with
// messages like "ssh: handshake failed: ssh: unable to authenticate, ..."
// and includes the list of methods the server advertised.
func translateDialError(hostport string, hasKeys bool, err error) error {
	if err == nil {
		return nil
	}
	var keErr *knownhosts.KeyError
	if errors.As(err, &keErr) && len(keErr.Want) > 0 {
		return fmt.Errorf("%w: %v", ErrHostKeyMismatch, err)
	}
	// Network-level failures (no route, DNS, refused connection) surface as
	// *net.OpError without an "ssh:" prefix in the message.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Errorf("%w (%s): %v", ErrHostUnreachable, hostport, err)
	}
	msg := err.Error()
	if strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "unable to authenticate") ||
		// Go's ssh client surfaces "unexpected message type 51" when the
		// server replies with USERAUTH_FAILURE during the kbd-interactive
		// flow without ever sending an INFO_REQUEST (i.e. the server
		// doesn't actually support kbd-interactive). For our purposes
		// that is just an authentication failure.
		strings.Contains(msg, "unexpected message type 51") {
		methods := parseAdvertisedMethods(msg)
		if len(methods) == 1 && methods[0] == "publickey" && !hasKeys {
			return fmt.Errorf("%w (host %s)", ErrAuthRequiredKey, hostport)
		}
		if len(methods) > 0 {
			return fmt.Errorf(
				"ssh authentication to %s failed (server allowed: %s): %w",
				hostport, strings.Join(methods, ","), err,
			)
		}
		return fmt.Errorf("ssh authentication to %s failed: %w", hostport, err)
	}
	return fmt.Errorf("ssh dial %s: %w", hostport, err)
}

// parseAdvertisedMethods extracts the comma-separated list of methods the
// server advertised from a Go ssh failure message of the form
//   "...attempted methods [<tried>], no supported methods remain"
// or
//   "ssh: handshake failed: ssh: unable to authenticate, attempted methods [...], no supported methods remain"
// We don't have direct access to the failure packet from a public API, so
// the message text is the only source of this information.
func parseAdvertisedMethods(msg string) []string {
	const marker = "attempted methods ["
	i := strings.Index(msg, marker)
	if i < 0 {
		return nil
	}
	rest := msg[i+len(marker):]
	end := strings.Index(rest, "]")
	if end < 0 {
		return nil
	}
	parts := strings.Split(rest[:end], " ")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "none" {
			continue
		}
		out = append(out, p)
	}
	return out
}


func defaultHostkeyCallback(override string) (ssh.HostKeyCallback, error) {
	knownHostsPath := override
	if knownHostsPath == "" {
		home, err := currentHomePath()
		if err != nil {
			return nil, err
		}
		knownHostsPath = path.Join(home, ".ssh/known_hosts")
	}
	hostkeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %s", knownHostsPath, err)
	}
	return hostkeyCallback, nil
}

func getCurrentUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get default user: %s", err)
	}
	return u.Username, nil
}

func usernameOrCurrent(uri workspaceapi.URI) (string, error) {
	if uri.User() != "" {
		return uri.User(), nil
	}
	return getCurrentUser()
}

func hostPortFromURI(uri workspaceapi.URI) string {
	hostname, port := uri.Hostname(), uri.Port()
	if port == "" {
		port = "22"
	}
	return fmt.Sprintf("%s:%s", hostname, port)
}

func currentHomePath() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to lookup current username: %s", err)
	}
	return u.HomeDir, nil
}

