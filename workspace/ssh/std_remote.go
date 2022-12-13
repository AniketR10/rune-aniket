package ssh

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
)

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
	client *ssh.Client
}

// used to adapt ssh.Session to Executor
type goSshSession struct {
	cmd  string
	args []string
	ses  *ssh.Session
}

func newStdRemote(cfg sshConfig, uri workspaceapi.URI) (
	remote, error,
) {
	username, err := usernameOrCurrent(uri)
	if err != nil {
		return nil, err
	}
	auths, err := authMethodsFromURI(cfg, uri)
	if err != nil {
		return nil, err
	}

	hostkeyCallback, err := defaultHostkeyCallback()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to ssh dial: %s", err)
	}
	return stdRemote{conn}, nil
}

func (s *goSshSession) Command(name string, arg ...string) (workspace.Pid, error) {
	if name == "" {
		return 0, errors.New("invalid empty command")
	}
	if s.cmd != "" {
		panic("Command called more than once on an ssh session")
	}
	s.cmd, s.args = name, arg
	return workspace.Pid(0), nil
}

func (s *goSshSession) Start(workspace.Pid) error {
	if s.cmd == "" {
		return errors.New("invalid ssh.Session Executor state: must call Command first")
	}
	return s.ses.Start(fmt.Sprintf("%s %s", s.cmd, strings.Join(s.args, " ")))
}

func (s *goSshSession) Signal(_ workspace.Pid, sig syscall.Signal) error {
	signal, ok := sigMap[sig]
	if !ok {
		return errors.New("unknown signal")
	}
	return s.ses.Signal(signal)
}

func (s *goSshSession) StderrPipe(workspace.Pid) (io.ReadCloser, error) {
	r, err := s.ses.StderrPipe()
	return io.NopCloser(r), err
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func (s *goSshSession) StdinPipe(workspace.Pid) (io.WriteCloser, error) {
	r, err := s.ses.StdinPipe()
	return nopWriteCloser{r}, err
}

func (s *goSshSession) StdoutPipe(workspace.Pid) (io.ReadCloser, error) {
	r, err := s.ses.StdoutPipe()
	return io.NopCloser(r), err
}

func (s *goSshSession) Wait(workspace.Pid) error {
	return s.ses.Wait()
}

func (r *goSshSession) Close() error {
	return r.ses.Close()
}

func (r stdRemote) NewSession() (workspace.Executor, error) {
	ses, err := r.client.NewSession()
	return &goSshSession{ses: ses}, err
}

func (r stdRemote) Close() error {
	return r.client.Close()
}

func authMethodsFromURI(config sshConfig, uri workspaceapi.URI) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod
	for _, key := range config.privateKeys {
		signer, err := privateKeySigner(key)
		if err != nil {
			return nil, err
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if pass, ok := uri.Password(); ok {
		auths = append(auths, ssh.Password(pass))
	}
	return auths, nil
}

func defaultHostkeyCallback() (ssh.HostKeyCallback, error) {
	home, err := currentHomePath()
	if err != nil {
		return nil, err
	}

	knownHostsPath := path.Join(home, ".ssh/known_hosts")
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

func readPassphrase(key string) (string, error) {
	fmt.Fprintf(os.Stdout, "Key %s requires a passphrase: ", key)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}

	password := string(bytePassword)
	return password, nil
}

func privateKeySigner(privateKeyPath string) (ssh.Signer, error) {
	privateKey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read private key file %s: %s",
			privateKeyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err == nil {
		return signer, nil
	}
	if _, ok := err.(*ssh.PassphraseMissingError); !ok {
		return nil, fmt.Errorf("could not parse private key at %s: %s",
			privateKeyPath, err)
	}

	// handle passhprase errors by reading password from stdin
	passphrase, err := readPassphrase(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not read passphrase for private key at %s: %s",
			privateKeyPath, err)
	}
	signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKey, []byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf("could not parse private key with passphrase at %s: %s",
			privateKeyPath, err)
	}
	return signer, nil
}
