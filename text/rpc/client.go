package rpc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"

	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const (
	gracefulShutdownWait  = 400 * time.Millisecond
	defaultFailureTimeout = 5 * time.Second
)

// Token wraps a browser.Token to satisfy editor.Handler.
type Token struct {
	browser.Token
	resource workspace.URI
}

// Resource satisfies Handler
func (t Token) Resource() workspace.URI {
	return t.resource
}

// Client satisfies text.Editor by calling a remote editor over grpc.
type Client struct {
	// resources invariant
	mu sync.Mutex

	broker  proto.MuxBroker
	browser *browser.Client
	cc      grpc.ClientConnInterface
	ed      EditorClient

	// event handler server resources. event handler servers are created on
	// calls to Subscribe.
	servers map[uint64]io.Closer
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	broker proto.MuxBroker, cc grpc.ClientConnInterface,
) *Client {
	ret := new(Client)
	ret.Init(broker, cc)
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(
	broker proto.MuxBroker, cc grpc.ClientConnInterface,
) {
	c.ed = NewEditorClient(cc)
	c.cc = cc
	c.broker = broker
	c.servers = make(map[uint64]io.Closer)
	c.browser = browser.NewClient(broker, cc)
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "text.Client").Logf(level, msg, args...)
}

func (c *Client) getServers() map[uint64]io.Closer {
	return c.servers
}

func (c *Client) safeForceCloseHandler(brokerID uint32, reason string) error {
	c.log(log.DebugLevel, "editor.Client.safeForceCloseHandler(%d, reason=%s)", brokerID, reason)
	_, err := proto.ForceCloseResource(c.broker, uint64(brokerID),
		c.getServers, &c.mu)
	return err
}

func (c *Client) serveHandler(h text.EventHandler) (uint32, error) {
	brokerID, srv, err := proto.AcceptAndServe(c.broker,
		func(handlerID uint32, srv proto.MuxServer) {
			s := newEventHandlerServer(h, func() {
				time.Sleep(gracefulShutdownWait)
				c.safeForceCloseHandler(handlerID, "editorEventHandlerServer.onExit")
			})
			RegisterEditorEventHandlerServer(srv.Registrar(), s)
		})
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.servers[uint64(brokerID)] = &handlerServerResource{srv: srv}
	return brokerID, nil
}

func (c *Client) serveCommandHandler(h text.CommandHandler) (uint32, error) {
	brokerID, srv, err := proto.AcceptAndServe(c.broker,
		func(handlerID uint32, srv proto.MuxServer) {
			s := newCommandServer(h, c.browser)
			RegisterCommandHandlerServer(srv.Registrar(), s)
		})
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.servers[uint64(brokerID)] = &handlerServerResource{srv: srv}
	return brokerID, nil
}

// Edit requests editor server to edit buf.
func (c *Client) Edit(file workspace.URI, buf *cell.Buffer) (text.Handler, error) {
	ctx := context.Background()
	req := NewEditRequest(file, buf)

	res, err := c.ed.Edit(ctx, &req)
	if err != nil {
		return nil, err
	}

	token := browser.Token{ID: uint64(res.GetHandlerId())}
	return Token{Token: token, resource: file}, nil
}

// Editor satisfies text.Editor
func (c *Client) Editor(file workspace.URI) (text.Handler, error) {
	ctx := context.Background()
	req := EditorRequest{ResourceName: NewURI(file)}

	res, err := c.ed.Editor(ctx, &req)
	if err != nil {
		return nil, err
	}

	token := browser.Token{ID: uint64(res.GetHandlerId())}
	return Token{Token: token, resource: file}, nil
}

// SubscribeEditorEvents requests the editor server to subscribe sub to ev.
func (c *Client) SubscribeEditorEvents(evs []text.EventType, h text.EventHandler) error {
	ctx := context.Background()

	handlerID, err := c.serveHandler(h)
	if err != nil {
		return fmt.Errorf("serveHandler: %w", err)
	}

	req := EditorSubscribeRequest{HandlerId: handlerID}
	for _, ev := range evs {
		req.Type = append(req.Type, protoType(text.Event{Type: ev}))
	}
	_, err = c.ed.Subscribe(ctx, &req)
	if err != nil {
		reason := fmt.Sprintf("editor.Client.Subscribe: %v", err)
		c.safeForceCloseHandler(handlerID, reason)
		return err
	}

	return nil
}

// SubscribeCommandrequests the editor server to register cmd with h.
func (c *Client) SubscribeCommand(cmd string, h text.CommandHandler) error {
	ctx := context.Background()

	// re-use EventHandler logic
	handlerID, err := c.serveCommandHandler(h)
	if err != nil {
		return fmt.Errorf("serveHandler: %w", err)
	}

	req := RegisterCommandRequest{Command: cmd, HandlerId: handlerID}
	_, err = c.ed.Register(ctx, &req)
	if err != nil {
		reason := fmt.Sprintf("editor.Client.Register: %v", err)
		c.safeForceCloseHandler(handlerID, reason)
		return err
	}

	return nil
}

func makeLocationListRequest(
	handlerID uint32, priority text.LocationPriority,
	listID string, l text.LocationList,
) SetLocationListRequest {
	req := SetLocationListRequest{
		HandlerId: handlerID,
		ListId:    listID,
		Priority:  uint32(priority),
	}

	for loc, ok := l.Current(); ok; loc, ok = l.Next() {
		var from, to termpb.Coordinates
		var attr termpb.Attributes
		from.FromModel(loc.From)
		to.FromModel(loc.To)
		attr.FromModel(loc.Attr)
		req.Locations = append(req.Locations, &SetLocationListRequest_Location{
			From: &from,
			To:   &to,
			Attr: &attr,
			Msg:  loc.Message,
		})
	}
	return req
}

// SetLocationList requests the editor server to set l as the new location list for h.
// Note that h is expected to be the return valu of Edit or a dispatched event, delivered
// via an EventHandler.
func (c *Client) SetLocationList(
	h text.Handler, pri text.LocationPriority, ID string, l text.LocationList,
) error {
	ctx := context.Background()
	token, ok := h.(Token)
	if !ok {
		panic("SetLocationList: invalid Handler argument")
	}
	req := makeLocationListRequest(uint32(token.ID), pri, ID, l)
	_, err := c.ed.SetLocationList(ctx, &req)
	return err
}

func (c *Client) moveToLocation(h text.Handler, ID string, next bool) (err error) {
	ctx := context.Background()
	token, ok := h.(Token)
	if !ok {
		panic("MoveToNextLocation: invalid Handler argument")
	}
	req := MoveToLocationRequest{HandlerId: uint32(token.ID), ListId: ID}
	if next {
		_, err = c.ed.MoveToNextLocation(ctx, &req)
	} else {
		_, err = c.ed.MoveToPrevLocation(ctx, &req)
	}
	return err
}

// MoveToPrevLocation requests the editor server to move cursor to the previous location
// in location list identified by ID.
func (c *Client) MoveToPrevLocation(h text.Handler, ID string) error {
	return c.moveToLocation(h, ID, false)
}

// MoveToNextLocation requests the editor server to move cursor to the next location
// in location list identified by ID.
func (c *Client) MoveToNextLocation(h text.Handler, ID string) error {
	return c.moveToLocation(h, ID, true)
}

// SetCursor requests the editor server to move cursor to pos
func (c *Client) SetCursor(h text.Handler, pos term.Coordinates) error {
	ctx := context.Background()
	token, ok := h.(Token)
	if !ok {
		panic("SetCursor: invalid Handler argument")
	}
	var protoPos termpb.Coordinates
	protoPos.FromModel(pos)
	req := SetCursorRequest{Pos: &protoPos, HandlerId: uint32(token.ID)}
	_, err := c.ed.SetCursor(ctx, &req)
	return err
}

// Cursor requests the editor server to move cursor to pos
func (c *Client) Cursor(h text.Handler) (term.Coordinates, error) {
	ctx := context.Background()
	token, ok := h.(Token)
	if !ok {
		panic("Cursor: invalid Handler argument")
	}
	req := CursorRequest{HandlerId: uint32(token.ID)}
	res, err := c.ed.Cursor(ctx, &req)
	if err != nil {
		return term.Coordinates{}, err
	}
	return res.GetPos().ToModel(), nil
}

// CellEditor satisfies text.Editor.
func (c *Client) CellEditor(h text.Handler) text.CellEditor {
	token, ok := h.(Token)
	if !ok {
		panic("SetLocationList: invalid Handler argument")
	}
	return clientWriter{client: c, handlerID: uint32(token.ID)}
}

// CellView satisfies text.Editor.
func (c *Client) CellView(h text.Handler) text.CellView {
	token, ok := h.(Token)
	if !ok {
		panic("CellView: invalid Handler argument")
	}
	return clientView{client: c, handlerID: uint32(token.ID)}
}

// SetDefaultAttributes satisfies text.Editor.
func (c *Client) SetDefaultAttributes(h text.Handler, attrs term.Attributes) error {
	ctx := context.Background()
	token, ok := h.(Token)
	if !ok {
		panic("SetDefaultAttributs: invalid Handler argument")
	}
	var rpcAttrs termpb.Attributes
	rpcAttrs.FromModel(attrs)
	req := SetDefaultAttributesRequest{
		HandlerId:  uint32(token.ID),
		Attributes: &rpcAttrs,
	}
	_, err := c.ed.SetDefaultAttributes(ctx, &req)
	return err
}

// Close closes all resources associated with this client.
func (c *Client) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	for _, res := range c.servers {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}

	c.servers = nil

	return err
}
