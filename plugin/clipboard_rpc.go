package plugin

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	pluginpb "unstable.build/go-tui/plugin/rpc"
	"unstable.build/go-tui/proto"

	"google.golang.org/grpc"
)

const (
	defaultFailureTimeout = 5 * time.Second
)

type clipboardServer struct {
	pluginpb.UnimplementedClipboardServer
	broker                proto.MuxBroker
	locker                sync.Locker
	clients               map[uint64]io.Closer
	c                     ClipboardSetter
	failureTimeout        time.Duration
	defaultRegisterServer *clipboardRegisterServer
}

type clipboardRegisterClient struct {
	cc            proto.MuxConn
	c             pluginpb.ClipboardRegisterClient
	cancelMonitor func()
	hook          func() error
}

func (c *clipboardRegisterClient) Paste() (string, time.Time, error) {
	ctx := context.Background()
	req := pluginpb.ClipboardPasteRequest{}

	res, err := c.c.Paste(ctx, &req)
	if err != nil {
		return "", time.Time{}, err
	}
	return res.GetData(), protoTimeToStd(res.GetTimestamp()), nil
}

func (c *clipboardRegisterClient) Copy(data string, ts time.Time) error {
	ctx := context.Background()
	req := pluginpb.ClipboardCopyRequest{Data: data, Timestamp: stdTimeToProto(ts)}

	_, err := c.c.Copy(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *clipboardRegisterClient) Close() (ret error) {
	if c.cancelMonitor == nil {
		return nil
	}
	if c.hook != nil {
		if err := c.hook(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	c.cancelMonitor()
	if err := c.cc.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	c.cancelMonitor = nil
	return
}

// newClipboardServer expects c, Clipboard to be safe to use concurrently
func newClipboardServer(
	broker proto.MuxBroker, c Clipboard,
	locker sync.Locker,
) *clipboardServer {
	ret := new(clipboardServer)
	ret.broker = broker
	ret.c = c
	ret.failureTimeout = defaultFailureTimeout
	ret.clients = make(map[uint64]io.Closer)
	ret.locker = locker
	// to satisfy ClipboardRegister. It doesn't need to be a property
	// of clipboardServer other than to tie together their lifecycles.
	ret.defaultRegisterServer = &clipboardRegisterServer{r: c, locker: locker}
	return ret
}

func (s *clipboardServer) getClients() map[uint64]io.Closer {
	return s.clients
}

func (s *clipboardServer) dialRegister(handlerID uint32) (ClipboardRegister, error) {
	handlerConn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	c := pluginpb.NewClipboardRegisterClient(handlerConn)
	client := &clipboardRegisterClient{c: c, cc: handlerConn}

	ctx, cancelFn := context.WithCancel(context.Background())
	go proto.MonitorConnection(ctx, s.failureTimeout, handlerConn, func(reason string) {
		_, _ = proto.ForceCloseResource(s.broker, uint64(handlerID),
			s.getClients, s.locker)
		s.locker.Lock()
		defer s.locker.Unlock()
		if client.hook != nil {
			client.hook()
		}
	})

	s.locker.Lock()
	defer s.locker.Unlock()

	client.cancelMonitor = cancelFn
	s.clients[uint64(handlerID)] = client

	return client, nil
}

func (s *clipboardServer) SetRegister(ctx context.Context, req *pluginpb.SetRegisterRequest) (
	*pluginpb.SetRegisterResponse, error,
) {
	handlerID := req.GetHandlerId()
	register, err := s.dialRegister(uint32(handlerID))
	if err != nil {
		return nil, err
	}

	s.locker.Lock()
	defer s.locker.Unlock()

	registerID := req.GetRegisterId()
	err = s.c.SetRegister(registerID, register)
	if err != nil {
		return nil, err
	}

	return new(pluginpb.SetRegisterResponse), nil
}

func (s *clipboardServer) Close() (ret error) {
	if s.c == nil {
		return
	}

	for _, c := range s.clients {
		if err := c.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}

	err := s.defaultRegisterServer.Close()
	if err != nil {
		ret = multierr.Append(ret, err)
	}

	// reference cycle
	s.c = nil

	return
}

type clipboardClient struct {
	mu        sync.Mutex
	registers map[string]*clipboardRegisterServer

	broker         proto.MuxBroker
	cc             grpc.ClientConnInterface
	c              pluginpb.ClipboardClient
	remoteRegister *clipboardRegisterClient
}

type clipboardRegisterServer struct {
	pluginpb.UnimplementedClipboardRegisterServer
	srv    proto.MuxServer
	r      ClipboardRegister
	locker sync.Locker
}

func (c *clipboardRegisterServer) Copy(
	ctx context.Context, req *pluginpb.ClipboardCopyRequest,
) (res *pluginpb.ClipboardCopyResponse, err error) {
	c.locker.Lock()
	defer c.locker.Unlock()

	data := req.GetData()
	err = c.r.Copy(data, protoTimeToStd(req.GetTimestamp()))
	if err != nil {
		return
	}
	res = new(pluginpb.ClipboardCopyResponse)
	return
}

func (c *clipboardRegisterServer) Paste(
	ctx context.Context, req *pluginpb.ClipboardPasteRequest,
) (res *pluginpb.ClipboardPasteResponse, err error) {
	c.locker.Lock()
	defer c.locker.Unlock()

	res = new(pluginpb.ClipboardPasteResponse)
	var ts time.Time
	res.Data, ts, err = c.r.Paste()
	if err != nil {
		res = nil
		return
	}
	res.Timestamp = stdTimeToProto(ts)
	return
}

func (c *clipboardRegisterServer) Close() error {
	if c.srv != nil {
		c.srv.Stop()
	}
	return nil
}

func newClipboardClient(
	broker proto.MuxBroker, cc proto.MuxConn,
) Clipboard {
	ret := new(clipboardClient)
	ret.broker = broker
	ret.cc = cc
	ret.c = pluginpb.NewClipboardClient(cc)
	ret.registers = make(map[string]*clipboardRegisterServer)

	c := pluginpb.NewClipboardRegisterClient(cc)
	ret.remoteRegister = &clipboardRegisterClient{c: c, cc: cc}

	return ret
}

func (c *clipboardClient) serveClipboardRegister(
	r ClipboardRegister,
) (*clipboardRegisterServer, uint32, error) {
	rs := &clipboardRegisterServer{r: r, locker: &c.mu}
	brokerID, srv, err := proto.AcceptAndServe(c.broker,
		func(handlerID uint32, srv proto.MuxServer) {
			pluginpb.RegisterClipboardRegisterServer(srv.GRPC(), rs)
		})
	if err != nil {
		return nil, 0, err
	}
	rs.srv = srv
	return rs, brokerID, nil
}

func (c *clipboardClient) register(registerID string) (ClipboardRegister, error) {
	panic("this should not be called")
}

func (c *clipboardClient) Paste() (string, time.Time, error) {
	return c.remoteRegister.Paste()
}

func (c *clipboardClient) Copy(data string, ts time.Time) error {
	return c.remoteRegister.Copy(data, ts)
}

func (c *clipboardClient) SetRegister(registerID string, r ClipboardRegister) error {
	c.mu.Lock()
	if c, ok := c.registers[registerID]; ok {
		_ = c.Close()
	}
	c.mu.Unlock()
	ctx := context.Background()
	cc, handlerID, err := c.serveClipboardRegister(r)
	if err != nil {
		return fmt.Errorf("serveClipboardRegister: %w", err)
	}
	req := pluginpb.SetRegisterRequest{HandlerId: uint64(handlerID), RegisterId: registerID}
	_, err = c.c.SetRegister(ctx, &req)
	if err != nil {
		_ = cc.Close()
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.registers[registerID] = cc

	return nil
}

func (c *clipboardClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cc := range c.registers {
		_ = cc.Close()
	}
	if closer, ok := c.cc.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
