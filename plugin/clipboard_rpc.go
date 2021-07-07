package plugin

import (
	"context"
	"time"
	"io"
	"sync"

	"github.com/ernestrc/go-tui/proto"
	log "github.com/sirupsen/logrus"

	"google.golang.org/grpc"
)

const (
	defaultFailureTimeout = 5 * time.Second
)

type clipboardSetterServer struct {
	broker proto.MuxBroker
	mu sync.Mutex
	clients map[uint64]io.Closer
	c ClipboardSetter
	failureTimeout time.Duration
	logger *log.Logger
}

type clipboardRegisterClient struct {
	cc proto.ClipboardRegisterClient
	cancelMonitor func()
}

func (c *clipboardRegisterClient) Paste() (string, error) {
	ctx := context.Background()
	req := proto.ClipboardPasteRequest{}

	res, err := c.cc.Paste(ctx, &req)
	if err != nil {
		return "", err
	}
	return res.GetData(), nil
}

func (c *clipboardRegisterClient) Copy(data string) error {
	ctx := context.Background()
	req := proto.ClipboardCopyRequest{Data: data}

	_, err := c.cc.Copy(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *clipboardRegisterClient) Close() error {
	if c.cancelMonitor == nil {
		return nil
	}
	c.cancelMonitor()
	return nil
}

func newClipboardSetterServer(
	logger *log.Logger, broker proto.MuxBroker, c ClipboardSetter,
) *clipboardSetterServer {
	ret := new(clipboardSetterServer)
	ret.broker = broker
	ret.c = c
	ret.logger = logger
	ret.failureTimeout = defaultFailureTimeout
	ret.clients = make(map[uint64]io.Closer)
	return ret
}

func (s *clipboardSetterServer) getClients() map[uint64]io.Closer {
	return s.clients
}

func (s *clipboardSetterServer) dialRegister(handlerID uint32) (ClipboardRegister, error) {
	handlerConn, err := s.broker.Dial(handlerID)
	if err != nil {
		return nil, err
	}

	cc := proto.NewClipboardRegisterClient(handlerConn)
	client := &clipboardRegisterClient{cc: cc}

	ctx, cancelFn := context.WithCancel(context.Background())
	go proto.MonitorConnection(ctx, s.failureTimeout, handlerConn, func(reason string) {
		_, _ = proto.ForceCloseResource(uint64(handlerID), s.getClients, s.logger, &s.mu)
	})

	client.cancelMonitor =  cancelFn

	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[uint64(handlerID)] = client

	return client, nil
}

func (s *clipboardSetterServer) SetRegister(ctx context.Context, req *proto.SetRegisterRequest) (
	*proto.SetRegisterResponse, error,
) {
	handlerID := req.GetHandlerId()
	register, err := s.dialRegister(uint32(handlerID))
	if err != nil {
		return nil, err
	}

	err = s.c.SetRegister(req.GetRegisterId(), register)
	if err != nil {
		return nil, err
	}

	return new(proto.SetRegisterResponse), nil
}

type clipboardSetterClient struct {
	mu sync.Mutex
	registers map[string]*clipboardRegisterServer

	broker proto.MuxBroker
	cc     grpc.ClientConnInterface
	c      proto.ClipboardClient
	logger *log.Logger

}

type clipboardRegisterServer struct {
	srv proto.MuxServer
	r ClipboardRegister
}

func (c *clipboardRegisterServer) Copy(
	ctx context.Context, req *proto.ClipboardCopyRequest,
) (res *proto.ClipboardCopyResponse, err error) {
	data := req.GetData()
	err = c.r.Copy(data)
	if err != nil {
		return
	}
	res = new(proto.ClipboardCopyResponse)
	return
}

func (c *clipboardRegisterServer) Paste(
	ctx context.Context, req *proto.ClipboardPasteRequest,
) (res *proto.ClipboardPasteResponse, err error) {
	res = new(proto.ClipboardPasteResponse)	
	res.Data, err = c.r.Paste()
	if err != nil {
		res = nil
		return
	}
	return
}

func (c *clipboardRegisterServer) Close() error {
	c.srv.Stop()
	return nil
}

func newClipboardSetterClient(
	logger *log.Logger, broker proto.MuxBroker, cc grpc.ClientConnInterface,
) ClipboardSetter {
	ret := new(clipboardSetterClient)
	ret.broker = broker
	ret.cc = cc
	ret.logger = logger
	ret.c = proto.NewClipboardClient(cc)
	ret.registers = make(map[string]*clipboardRegisterServer)
	return ret
}

func (c *clipboardSetterClient) serveClipboardRegister(
	r ClipboardRegister,
) (*clipboardRegisterServer, uint32) {
	rs := &clipboardRegisterServer{r :r}
	brokerID, srv := proto.AcceptAndServe(c.broker, c.logger,
		func(handlerID uint32, srv proto.MuxServer) {
			proto.RegisterClipboardRegisterServer(srv.GRPC(), rs)
		})
	rs.srv = srv
	return rs, brokerID
}

func (c *clipboardSetterClient) SetRegister(registerID string, r ClipboardRegister) error {
	c.mu.Lock()
	if c, ok := c.registers[registerID]; ok {
		_ = c.Close()
	}
	c.mu.Unlock()
	ctx := context.Background()
	cc, handlerID := c.serveClipboardRegister(r)
	req := proto.SetRegisterRequest{HandlerId: uint64(handlerID), RegisterId: registerID}
	_, err := c.c.SetRegister(ctx, &req)
	if err != nil {
		_ = cc.Close()
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.registers[registerID] = cc

	return nil
}

func (c *clipboardSetterClient) Close() error {
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
