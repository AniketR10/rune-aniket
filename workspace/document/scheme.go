package document

import (
	"errors"

	"github.com/ernestrc/blue/document"
)

var (
	errExecute = errors.New("cannot execute commands on document.Service backed workspace.Scheme")
)

type scheme struct {
	uriScheme string
	svc       document.Service
}

/*
// TODO allow clients to add a document mapper to get the "pretty" file name.
// NewScheme returns a workspace.Scheme backed by the given document.Service.
func NewSchemeBuilder(svc document.Service, scheme string) workspace.SchemeFunc {
	return func(config config.Config, uri workspace.URI) (workspace.Scheme, error) {
		return &scheme{uriScheme: scheme, db: db}
	}
}

func (s *scheme) URI(path string) (workspace.URI, error) {
	absPath, err := s.expandPath(path)
	if err != nil {
		return workspace.URI{}, err
	}
	if err != nil {
		return workspace.URI{}, err
	}
	uriStr := fmt.Sprintf("%s://%s@%s%s", s.uriScheme, s.uri.User(), s.uri.Host(), absPath)
	return workspace.ParseURI(uriStr)
}

func (s *scheme) Open(path string, flag int, perm os.FileMode) (workspace.File, *workspace.Error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, workspace.NopError(err)
	}
	f, err := blupspin.Open(s.client, uname, flag)
	if err != nil {
		return nil, mapUpspinError(err)
	}
	if flag&os.O_CREATE != 0 {
		// make sure entry is created
		err := f.Sync()
		if err != nil {
			return nil, mapUpspinError(err)
		}
	}
	return fileAdapter{client: s.client, file: f}, nil
}

func (s *scheme) Remove(path string) error {
	return s.db.Delete(path)
}

func (s *scheme) Rename(oldpath, newpath string) error {
}

func (s *scheme) Stat(path string) (os.FileInfo, error) {
}

func (s *scheme) Lstat(path string) (os.FileInfo, error) {
}

func (s *scheme) ReadLink(path string) (string, error) {
}

func (s *scheme) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
}

func (s *scheme) Command(name string, arg ...string) (workspace.Pid, error) {
	return 0, errExecute
}

func (s *scheme) Start(workspace.Pid) error {
	return errExecute
}

func (s *scheme) Signal(workspace.Pid, syscall.Signal) error {
	return errExecute
}

func (s *scheme) StderrPipe(workspace.Pid) (io.ReadCloser, error) {
	return nil, errExecute
}

func (s *scheme) StdinPipe(workspace.Pid) (io.WriteCloser, error) {
	return nil, errExecute
}

func (s *scheme) StdoutPipe(workspace.Pid) (io.ReadCloser, error) {
	return nil, errExecute
}

func (s *scheme) Wait(workspace.Pid) error {
	return errExecute
}

func (s *scheme) NewPty() (workspace.Pty, error) {
	return workspace.Pty{}, errExecute
}

func (s *scheme) SetPtySize(workspace.Pty, int, int) error {
	return errExecute
}

func (s *scheme) Close() error {
	return nil
}
*/
