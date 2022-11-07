package plugin

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pluginpb "unstable.build/go-tui/plugin/rpc"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
)

const (
	// PermissionClipboard requests access to the editor.
	PermissionClipboard Permission = "_PermClipboard"
)

// ClipboardRegister is the interface that wraps the basic Copy, Paste methods
// for terminal applications that use multiple "registers" for short-lived text storage.
type ClipboardRegister interface {
	Paste() (string, time.Time, error)
	Copy(string, time.Time) error
}

// ClipboardSetter is the interface that wraps the basic methods SetRegister.
type ClipboardSetter interface {
	// SetRegister sets the register to be used for registerID. If there's already
	// a register installed for registerID, then it should be overwritten.
	SetRegister(registerID string, r ClipboardRegister) error
}

// Clipboard combines a ClipboardSetter with a ClipboardRegister.
type Clipboard interface {
	ClipboardSetter

	// ClipboardRegister for the default registerID
	ClipboardRegister
}

// ClipboardManager satisfies text.Clipboard by means of a plugin.ClipboardSetter
// which can be used to install arbitrary plugin.ClipboardRegister implementations.
type ClipboardManager struct {
	s *clipboardServer
	// text.Clipboard is re-used but each implementation is only
	// used for its registered registerID.
	registers map[string]*pluginRegister
}

// NewClipboardManager allocates storage for a new ClipboardManager and initializes it.
func NewClipboardManager() *ClipboardManager {
	ret := new(ClipboardManager)
	ret.Init()
	return ret
}

// avoid conflict of Copy/Paste methods
type clipboardManagerServer struct {
	*ClipboardManager
	lastUpdated time.Time
}

// Init initializes this ClipboardManager.
func (s *ClipboardManager) Init() {
	s.registers = make(map[string]*pluginRegister)
}

// Serve satisfies ResourceServer.
func (s *ClipboardManager) Serve(
	pluginID string, grantID uint32, broker proto.MuxBroker,
	lock sync.Locker,
) error {
	return acceptAndServe(broker, grantID, func(opts []grpc.ServerOption) proto.MuxServer {
		lock.Lock()
		defer lock.Unlock()

		// create a new server every time Serve is called
		// so ClipboardManager can be shared across workspaces
		var srv proto.MuxServer
		if log.IsLevelEnabled(log.TraceLevel) {
			srv = proto.LoggingGRPCServer(opts...)
		} else {
			srv = proto.GRPCServer(opts...)
		}
		grpc := srv.GRPC()

		// uses this ClipboardManager as the clipboard implementation
		// for all resource requests.
		s.s = newClipboardServer(broker, &clipboardManagerServer{ClipboardManager: s}, lock)
		pluginpb.RegisterClipboardServer(grpc, s.s)
		pluginpb.RegisterClipboardRegisterServer(grpc, s.s.defaultRegisterServer)

		return srv
	})
}

func (s *clipboardManagerServer) SetRegister(
	registerID string, r ClipboardRegister,
) error {
	return s.ClipboardManager.SetRegister(registerID, r)
}

func (s *clipboardManagerServer) Paste() (d string, updated time.Time, err error) {
	data, err := s.ClipboardManager.Paste(text.DefaultRegisterID)
	if err != nil {
		return "", time.Time{}, err
	}
	return data.Text, s.lastUpdated, nil
}

func (s *clipboardManagerServer) Copy(data string, timestamp time.Time) error {
	d := text.ClipboardData{Text: data}
	err := s.ClipboardManager.Copy(text.DefaultRegisterID, d)
	if err == nil {
		s.lastUpdated = timestamp
	}
	return err
}

func (s *ClipboardManager) log(msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "plugin.ClipboardManager").
		Tracef(msg, args...)
}

// Paste satisfies ClipboardRegister.
func (s *ClipboardManager) Paste(registerID string) (d text.ClipboardData, err error) {
	reg, ok := s.registers[registerID]
	if !ok {
		return
	}

	d, err = reg.Paste(registerID)
	s.log("Paste(%s): %#v, %#v", registerID, d, err)
	return
}

func newMultiRegister(initial ClipboardRegister) (*pluginRegister, string) {
	multi := newMultiClipboard()
	token := multi.add(initial)
	reg := newPluginRegister(multi)
	return reg, token
}

// Copy satisfies ClipboardRegister.
func (s *ClipboardManager) Copy(registerID string, data text.ClipboardData) error {
	reg, ok := s.registers[registerID]
	if !ok {
		adapter := newTextRegister(registerID, text.NewInMemoryClipboard())
		reg, _ = newMultiRegister(adapter)
		s.registers[registerID] = reg
	}

	err := reg.Copy(registerID, data)
	s.log("Copy(%s, %#v): %#v", registerID, data, err)
	return err
}

func (s *ClipboardManager) tryAddCloseHook(
	r ClipboardRegister, rm *pluginRegister, token string,
) {
	if client, ok := r.(*clipboardRegisterClient); ok {
		// hook should only be called via calls to ClipboardManager.Close(),
		// or via connection monitoring, in which case both
		// should already be synchronizing over state
		client.hook = func() error {
			rm.r.(*multiClipboard).remove(token)
			return nil
		}
	}
}

// SetRegister satisfies ClipboardSetter.
func (s *ClipboardManager) SetRegister(registerID string, r ClipboardRegister) error {
	if registerID == "" {
		return errors.New("Invalid registerID")
	}
	pr, ok := s.registers[registerID]
	if !ok {
		rm, token := newMultiRegister(r)
		s.registers[registerID] = rm
		s.tryAddCloseHook(r, rm, token)
		s.log("SetRegister(%s, %#v): created new multi register, token=%s",
			registerID, r, token)
		return nil
	}

	if multi, ok := pr.r.(*multiClipboard); ok {
		token := multi.add(r)
		s.tryAddCloseHook(r, pr, token)
		s.log("SetRegister(%s, %#v): added register to multi register, token=%s",
			registerID, r, token)
		return nil
	}

	// replace with multi
	mr, token := newMultiRegister(pr.r)
	s.registers[registerID] = mr
	s.tryAddCloseHook(r, mr, token)
	s.log("SetRegister(%s, %#v): register was not multi; replacing with multi, token=%s",
		registerID, r, token)
	return nil
}

// Close releases all resources associated with this ClipboardManager.
func (s *ClipboardManager) Close() (ret error) {
	if s.s != nil {
		s.s.Close()
	}
	for _, r := range s.registers {
		if err := r.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}

	return
}

// adapts a text.Clipboard into ClipboardRegister
type textRegister struct {
	registerID  string
	lastUpdated time.Time
	r           text.Clipboard
}

func newTextRegister(registerID string, r text.Clipboard) *textRegister {
	ret := new(textRegister)
	ret.r = r
	ret.registerID = registerID
	return ret
}

func (r *textRegister) Paste() (d string, updated time.Time, err error) {
	data, err := r.r.Paste(r.registerID)
	if err != nil {
		return "", time.Time{}, err
	}
	return data.Text, r.lastUpdated, nil
}

func (r *textRegister) Copy(data string, timestamp time.Time) error {
	err := r.r.Copy(r.registerID, text.ClipboardData{Text: data})
	if err == nil {
		r.lastUpdated = timestamp
	}
	return err
}

func (r *textRegister) Close() error {
	return nil
}

// adapts a ClipboardRegister into text.Clipboard
type pluginRegister struct {
	// plugins cannot provide storage for text.ClipboardData's empty interface field
	metadata interface{}
	r        ClipboardRegister
	updated  time.Time
}

func newPluginRegister(r ClipboardRegister) *pluginRegister {
	ret := new(pluginRegister)
	ret.r = r
	return ret
}

func (r *pluginRegister) Paste(registerID string) (d text.ClipboardData, err error) {
	var ts time.Time
	d.Text, ts, err = r.r.Paste()
	if err != nil {
		return
	}
	if ts.Equal(r.updated) {
		d.Metadata = r.metadata
	}
	return
}

func (r *pluginRegister) Copy(registerID string, data text.ClipboardData) error {
	now := time.Now()
	err := r.r.Copy(data.Text, now)
	if err != nil {
		return err
	}
	r.updated = now
	r.metadata = data.Metadata
	return nil
}

func (r *pluginRegister) Close() error {
	if closer, ok := r.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func dialClipboard(token uint32, broker proto.MuxBroker) (
	Clipboard, error,
) {
	conn, err := broker.Dial(token)
	if err != nil {
		return nil, err
	}
	c := newClipboardClient(broker, conn)
	return c, nil
}

// GetClipboard acquires the remote plugin.ClipboardSetter
// with the given permission token and broker.
func GetClipboard(token uint32, broker proto.MuxBroker) (
	Clipboard, error,
) {
	return dialClipboard(token, broker)
}
