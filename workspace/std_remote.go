package workspace

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
type goSshClient struct {
	client *ssh.Client
}

// used to adapt ssh.Session to Executor
type goSshSession struct {
	cmd  string
	args []string
	ses  *ssh.Session
}

func (s goSshSession) Command(name string, arg ...string) (Pid, error) {
	s.cmd, s.args = name, arg
	return Pid(0), nil
}

func (s *goSshSession) Start(Pid) error {
	if s.cmd == "" {
		return errors.New("invalid ssh.Session Executor state: must call Command first")
	}
	return s.ses.Start(fmt.Sprintf("%s %s", s.cmd, strings.Join(s.args, " ")))
}

func (s *goSshSession) Signal(_ Pid, sig syscall.Signal) error {
	signal, ok := sigMap[sig]
	if !ok {
		return errors.New("unknown signal")
	}
	return s.ses.Signal(signal)
}

func (s *goSshSession) StderrPipe(Pid) (io.ReadCloser, error) {
	r, err := s.ses.StderrPipe()
	return io.NopCloser(r), err
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func (s *goSshSession) StdinPipe(Pid) (io.WriteCloser, error) {
	r, err := s.ses.StdinPipe()
	return nopWriteCloser{r}, err
}

func (s *goSshSession) StdoutPipe(Pid) (io.ReadCloser, error) {
	r, err := s.ses.StdoutPipe()
	return io.NopCloser(r), err
}

func (s *goSshSession) Wait(Pid) error {
	return s.ses.Wait()
}

func (r goSshClient) NewSession() (Executor, error) {
	ses, err := r.client.NewSession()
	return &goSshSession{ses: ses}, err
}

func (r goSshClient) Close() error {
	return r.client.Close()
}

func authMethodsFromURI(config managerCfg, workspace URI) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod
	for _, key := range config.sshPrivateKeys {
		signer, err := privateKeySigner(key)
		if err != nil {
			return nil, err
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if pass, ok := workspace.parsed.User.Password(); ok {
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

func usernameFromURI(workspace URI) (string, error) {
	if workspace.parsed.User != nil && workspace.parsed.User.Username() != "" {
		return workspace.parsed.User.Username(), nil
	}
	return getCurrentUser()
}

func (m *Manager) connectOverStdSSH() (
	sshClient, error,
) {
	username, err := usernameFromURI(m.workspace)
	if err != nil {
		return nil, err
	}
	auths, err := authMethodsFromURI(m.managerCfg, m.workspace)
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
		Timeout:         m.managerCfg.sshTimeout,
	}

	hostport := hostPortFromURI(m.workspace)
	conn, err := ssh.Dial("tcp", hostport, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to ssh dial: %s", err)
	}
	return goSshClient{conn}, nil
}

func hostPortFromURI(u URI) string {
	hostname, port := u.parsed.Hostname(), u.parsed.Port()
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
