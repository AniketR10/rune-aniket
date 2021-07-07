package plugin

//go:generate mockgen -destination=./clipboard_gomock.go -package plugin -self_package plugin -source clipboard.go

import (
	"errors"
	"sync"

	"github.com/ernestrc/go-tui/editor"
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

// ClipboardSetter is the interface that wraps the basic method to install new clipboard
// registers, SetRegister. Plugin implementors can use the method
type ClipboardSetter interface {
	// SetRegister sets the register to be used for registerID. If there's already
	// a register installed for registerID, then it should be overwritten.
	SetRegister(registerID string, r ClipboardRegister) error
}

// ClipboardManager satisfies editor.Clipboard by means of a plugin.ClipboardSetter
// which can be used to install arbitrary plugin.ClipboardRegister implementations.
type ClipboardManager struct {
	mu  sync.Mutex
	srv proto.MuxServer
	// editor.Clipboard is re-used but each implementation is only
	// used for its registered registerID.
	registers map[string]editor.Clipboard
}

// NewClipboardManager allocates storage for a new ClipboardManager and initializes it.
func NewClipboardManager() *ClipboardManager {
	ret := new(ClipboardManager)
	ret.Init()
	return ret
}

// Init initializes this ClipboardManager.
func (s *ClipboardManager) Init() {
	s.registers = make(map[string]editor.Clipboard)
	s.registers[editor.DefaultRegisterID] = editor.NewInMemoryClipboard()
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
		proto.RegisterClipboardServer(grpc, newClipboardSetterServer(l, broker, s))

		return s.srv
	})
}

// ResourceServer returns this ClipboardManager's ResourceServer,
// capable of serving PermissionClipboard.
func (s *ClipboardManager) ResourceServer() ResourceServer {
	return s
}

// Paste satisfies ClipboardRegister.
func (s *ClipboardManager) Paste(registerID string) (d editor.ClipboardData, err error) {
	s.mu.Lock()
	reg, ok := s.registers[registerID]
	s.mu.Unlock()
	if !ok {
		return
	}

	return reg.Paste("")
}

// Copy satisfies ClipboardRegister.
func (s *ClipboardManager) Copy(registerID string, data editor.ClipboardData) error {
	s.mu.Lock()
	reg, ok := s.registers[registerID]
	if !ok {
		reg = editor.NewInMemoryClipboard()
		s.registers[registerID] = reg
	}
	s.mu.Unlock()

	return reg.Copy("", data)
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

type pluginRegister struct {
	// plugins cannot provide storage for editor.ClipboardData's empty interface field
	metadata interface{}
	r        ClipboardRegister
}

func (r *pluginRegister) Paste(registerID string) (d editor.ClipboardData, err error) {
	d.Text, err = r.r.Paste()
	if err != nil {
		return
	}
	d.Metadata = r.metadata
	return
}

func (r *pluginRegister) Copy(registerID string, data editor.ClipboardData) error {
	err := r.r.Copy(data.Text)
	if err != nil {
		return err
	}
	r.metadata = data.Metadata
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
