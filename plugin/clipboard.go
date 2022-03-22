package plugin

//go:generate mockgen -destination=./clipboard_gomock_test.go -package plugin -self_package plugin -source clipboard.go

import (
	"errors"
	"io"
	"sync"

	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	// PermissionClipboard requests access to the editor.
	PermissionClipboard Permission = "_PermClipboard"
)

// ClipboardRegister is the interface that wraps the basic Copy, Paste methods
// for terminal applications that use multiple "registers" for short-lived text storage.
type ClipboardRegister interface {
	Paste() (string, error)
	Copy(string) error
}

// ClipboardSetter is the interface that wraps the basic methods Register and SetRegister.
type ClipboardSetter interface {
	// register returns the register registered with registerID or nil if there's
	// no register with that id.
	register(registerID string) (ClipboardRegister, error)

	// SetRegister sets the register to be used for registerID. If there's already
	// a register installed for registerID, then it should be overwritten.
	SetRegister(registerID string, r ClipboardRegister) error
}

// ClipboardManager satisfies editor.Clipboard by means of a plugin.ClipboardSetter
// which can be used to install arbitrary plugin.ClipboardRegister implementations.
type ClipboardManager struct {
	mu  sync.Mutex
	srv proto.MuxServer
	s   *clipboardSetterServer
	// editor.Clipboard is re-used but each implementation is only
	// used for its registered registerID.
	registers map[string]text.Clipboard
}

// NewClipboardManager allocates storage for a new ClipboardManager and initializes it.
func NewClipboardManager() *ClipboardManager {
	ret := new(ClipboardManager)
	ret.Init()
	return ret
}

// Init initializes this ClipboardManager.
func (s *ClipboardManager) Init() {
	s.registers = make(map[string]text.Clipboard)
	s.registers[text.DefaultRegisterID] = text.NewInMemoryClipboard()
}

// Serve satisfies ResourceServer.
func (s *ClipboardManager) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	l *log.Logger, lock sync.Locker,
) {
	broker.AcceptAndServe(grantID, func(opts []grpc.ServerOption) proto.MuxServer {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.srv != nil {
			return s.srv
		}

		var srv proto.MuxServer
		if l != nil && l.IsLevelEnabled(log.TraceLevel) {
			srv = proto.LoggingGRPCServer(l, opts...)
		} else {
			srv = proto.GRPCServer(opts...)
		}
		grpc := srv.GRPC()
		s.srv = srv

		// uses this ClipboardManager as the clipboard implementation
		// for all resource requests.
		s.s = newClipboardSetterServer(l, broker, s)
		proto.RegisterClipboardServer(grpc, s.s)

		return s.srv
	})
}

// Paste satisfies ClipboardRegister.
func (s *ClipboardManager) Paste(registerID string) (d text.ClipboardData, err error) {
	s.mu.Lock()
	reg, ok := s.registers[registerID]
	s.mu.Unlock()
	if !ok {
		return
	}

	return reg.Paste("")
}

// Copy satisfies ClipboardRegister.
func (s *ClipboardManager) Copy(registerID string, data text.ClipboardData) error {
	s.mu.Lock()
	reg, ok := s.registers[registerID]
	if !ok {
		reg = text.NewInMemoryClipboard()
		s.registers[registerID] = reg
	}
	s.mu.Unlock()

	return reg.Copy("", data)
}

// register satisfies ClipboardSetter.
func (s *ClipboardManager) register(registerID string) (ClipboardRegister, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.registers[registerID]
	if !ok {
		return nil, nil
	}
	if plugReg, ok := r.(*pluginRegister); ok {
		return plugReg.r, nil
	}
	return nil, nil
}

// SetRegister satisfies ClipboardSetter.
func (s *ClipboardManager) SetRegister(registerID string, r ClipboardRegister) error {
	if registerID == "" {
		return errors.New("Invalid registerID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registers[registerID] = &pluginRegister{r: r}
	return nil
}

func (s *ClipboardManager) Close() (ret error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, r := range s.registers {
		if closer, ok := r.(io.Closer); ok {
			err := closer.Close()
			if err != nil {
				ret = err
			}
		}
	}

	if s.s != nil {
		err := s.s.Close()
		if err != nil {
			ret = err
		}
	}
	if s.srv != nil {
		s.srv.Stop()
	}
	return
}

type pluginRegister struct {
	// plugins cannot provide storage for editor.ClipboardData's empty interface field
	metadata interface{}
	r        ClipboardRegister
}

func (r *pluginRegister) Paste(registerID string) (d text.ClipboardData, err error) {
	d.Text, err = r.r.Paste()
	if err != nil {
		return
	}
	d.Metadata = r.metadata
	return
}

func (r *pluginRegister) Copy(registerID string, data text.ClipboardData) error {
	err := r.r.Copy(data.Text)
	if err != nil {
		return err
	}
	r.metadata = data.Metadata
	return nil
}

func (r *pluginRegister) Close() error {
	if closer, ok := r.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func dialClipboardSetter(token uint32, broker proto.MuxBroker) (
	ClipboardSetter, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := newClipboardSetterClient(&pluginLogger, broker, conn)
	return c, nil
}

// Clipboard acquires the remote plugin.ClipboardSetter
// with the given permission token and broker.
func Clipboard(token uint32, broker proto.MuxBroker) (
	ClipboardSetter, error,
) {
	return dialClipboardSetter(token, broker)
}
