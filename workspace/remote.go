package workspace

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	os "os"
	"os/user"
	"path"
	"syscall"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
	"google.golang.org/grpc"
)

const (
	sshScheme = "ssh"
)

func (m *Manager) newOsRemoteFile() *file {
	ret := new(file)
	ret.openFunc = func(path string, flag int, perm os.FileMode) (osFile, *osError) {
		m.mu.Lock()
		err := m.sshErr
		m.mu.Unlock()
		if err != nil {
			return nil, nopOsError(err)
		}
		f, err := m.workspaceClient.Open(path, flag, perm)
		if err != nil {
			return nil, err.(*osError)
		}
		return f, nil
	}
	ret.removeFunc = func(path string) error {
		m.mu.Lock()
		err := m.sshErr
		m.mu.Unlock()
		if err != nil {
			return err
		}
		return m.workspaceClient.Remove(path)
	}
	ret.renameFunc = func(oldpath, newpath string) error {
		m.mu.Lock()
		err := m.sshErr
		m.mu.Unlock()
		if err != nil {
			return err
		}
		return m.workspaceClient.Rename(oldpath, newpath)
	}
	ret.statFunc = func(path string) (os.FileInfo, error) {
		m.mu.Lock()
		err := m.sshErr
		m.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return m.workspaceClient.Stat(path)
	}
	ret.lstatFunc = func(path string) (os.FileInfo, error) {
		m.mu.Lock()
		err := m.sshErr
		m.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return m.workspaceClient.LStat(path)
	}
	ret.readLinkFunc = func(path string) (string, error) {
		m.mu.Lock()
		err := m.sshErr
		m.mu.Unlock()
		if err != nil {
			return "", err
		}
		return m.workspaceClient.ReadLink(path)
	}
	return ret
}

func getCurrentUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get default user: %s", err)
	}
	return u.Username, nil
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

func usernameFromURI(workspace URI) (string, error) {
	if workspace.parsed.User != nil && workspace.parsed.User.Username() != "" {
		return workspace.parsed.User.Username(), nil
	}
	return getCurrentUser()
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

func connectOverSSH(config managerCfg, workspace URI) (*ssh.Client, error) {
	username, err := usernameFromURI(workspace)
	if err != nil {
		return nil, err
	}

	auths, err := authMethodsFromURI(config, workspace)
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
		Timeout:         config.sshTimeout,
	}

	hostport := hostPortFromURI(workspace)
	conn, err := ssh.Dial("tcp", hostport, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to ssh dial: %s", err)
	}
	return conn, nil
}

func (m *Manager) initWorkspaceClient() error {
	ses, err := m.sshConn.NewSession()
	if err != nil {
		return err
	}
	stdout, err := ses.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not get stdout pipe: %s", err)
	}
	stdin, err := ses.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not get stdin pipe: %s", err)
	}
	stderr, err := ses.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not get stderr pipe: %s", err)
	}

	path := m.workspace.Path()
	if path == "" {
		path = "."
	}

	// expects six to be in the path of the user
	err = ses.Start(fmt.Sprintf("six -x %s", path))
	if err != nil {
		return fmt.Errorf("could not start remote six: %s", err)
	}

	conn, err := grpc.Dial("", grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return newStdConn(stdout, stdin, func() {
				m.mu.Lock()
				defer m.mu.Unlock()
				if m.sshErr == nil {
					m.sshErr = errors.New("ssh connection closed unexpectedly")
				}
			}), nil
		}))
	if err != nil {
		return err
	}
	m.workspaceClient = NewClient(conn)

	go func() {
		err := ses.Wait()
		if err != nil {
			stderrStr, rerr := ioutil.ReadAll(stderr)
			m.mu.Lock()
			defer m.mu.Unlock()
			if rerr != nil {
				m.sshErr = fmt.Errorf("could not read error from stderr but there was"+
					"an error executing remote six server over SSH: %s", err)
			} else {
				m.sshErr = fmt.Errorf("error executing remote six server over SSH: %s: %s", err, stderrStr)
			}
		}
	}()
	return nil
}

func checkFileIsFromWorkspace(file, workspace URI) error {
	if file.parsed.Host != workspace.parsed.Host ||
		file.parsed.User.String() != workspace.parsed.User.String() {
		return errors.New("remote file's does not match remote workspace connection's user or host")
	}
	return nil
}

func (m *Manager) openRemoteFile(
	file URI, buf *cell.Buffer, swapDir URI, readOnly bool,
) (FlusherCloser, error) {
	if m.sshConn == nil {
		return nil, errors.New("cannot open remote file without a connection to a remote workspace")
	}
	err := checkFileIsFromWorkspace(file, m.workspace)
	if err != nil {
		return nil, err
	}
	f := m.newOsRemoteFile()
	err = f.init(file.Path(), buf, swapDir.Path(), readOnly)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (m *Manager) recoverRemoteFile(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	if m.sshConn == nil {
		return nil, errors.New("cannot open remote file without a connection to a remote workspace")
	}
	err := checkFileIsFromWorkspace(file, m.workspace)
	if err != nil {
		return nil, err
	}
	err = checkFileIsFromWorkspace(swapFile, m.workspace)
	if err != nil {
		return nil, err
	}
	f := m.newOsRemoteFile()
	err = f.recoverFile(file.Path(), swapFile.Path(), buf)
	if err != nil {
		return nil, err
	}
	return f, nil
}
