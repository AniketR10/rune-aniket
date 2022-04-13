package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	os "os"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"google.golang.org/grpc"
)

const (
	sshScheme = "ssh"
)

type sshClient interface {
	NewSession() (Executor, error)
	Close() error
}

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

func startProc(exec Executor, pid Pid) (
	stdout io.ReadCloser, stderr io.ReadCloser, stdin io.WriteCloser, err error,
) {
	stdout, err = exec.StdoutPipe(pid)
	if err != nil {
		err = fmt.Errorf("could not get stdout pipe: %s", err)
		return
	}

	stdin, err = exec.StdinPipe(pid)
	if err != nil {
		err = fmt.Errorf("could not get stdin pipe: %s", err)
		return
	}

	stderr, err = exec.StderrPipe(pid)
	if err != nil {
		err = fmt.Errorf("could not get stderr pipe: %s", err)
		return
	}

	err = exec.Start(pid)
	if err != nil {
		err = fmt.Errorf("could not start remote command: %s", err)
		return
	}
	return
}

func (m *Manager) initWorkspaceClient(config managerCfg, workspace URI) (err error) {

	m.isInitProxy = true

	if config.sshCommand == "" {
		m.sshConn, err = m.connectOverStdSSH()
	} else {
		m.sshConn, err = m.connectOverProcSSH()
	}
	if err != nil {
		return err
	}

	path := m.workspace.Path()
	if path == "" {
		path = "."
	}

	ses, err := m.sshConn.NewSession()
	if err != nil {
		return err
	}

	pid, err := ses.Command(fmt.Sprintf("six -x %s", path))
	if err != nil {
		return fmt.Errorf("could not create command: %s", err)
	}

	stdout, stderr, stdin, err := startProc(ses, pid)
	if err != nil {
		return err
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
		err := ses.Wait(pid)
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

	m.isInitProxy = false

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

// RemoteURI builds a URI with the current workspace and the given path.
func (m *Manager) remoteURI(path string) (URI, error) {
	if m.sshConn == nil {
		return URI{}, errors.New("cannot make a remote URI without a connection to a remote workspace")
	}
	absPath, err := m.extractAbsPath(path)
	if err != nil {
		return URI{}, err
	}
	uriStr := fmt.Sprintf("ssh://%s@%s%s",
		m.workspace.parsed.User.Username(), m.workspace.parsed.Host, absPath)
	u, err := url.Parse(uriStr)
	if err != nil {
		return URI{}, err
	}

	return makeSSHURI(u), nil
}
