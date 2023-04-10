package rpc

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
)

const (
	defaultTimeout = 2 * time.Second
)

// Token wraps a browser.Token to satisfy editor.Handler.
type Token struct {
	browser.Token
	workspaceapi.URI
}

// Resource satisfies Handler
func (t Token) Resource() workspaceapi.URI {
	return t.URI
}

var _ textapi.Editor = (*Client)(nil)

// Client satisfies text.Editor by calling a remote editor over grpc.
type Client struct {
	broker          proto.MuxBroker
	browser         *browserpb.Client
	cc              proto.MuxConn
	ed              EditorClient
	clientCtx       context.Context
	clientCancelCtx func()
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	broker proto.MuxBroker, cc proto.MuxConn,
) *Client {
	ret := new(Client)
	ret.Init(broker, cc)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(
	broker proto.MuxBroker, cc proto.MuxConn,
) {
	c.ed = NewEditorClient(cc)
	c.cc = cc
	c.broker = broker
	c.browser = browserpb.NewClient(broker, cc)
	c.clientCtx, c.clientCancelCtx = context.WithCancel(context.Background())
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "text.Client").Logf(level, msg, args...)
}

func (c *Client) serveCommandHandler(h textapi.CommandHandler) (
	ret string, srv proto.MuxServer, err error,
) {
	ret, err = proto.AcceptAndServeChannel(c.clientCtx, c.broker,
		func(channelID string, _srv proto.MuxServer) {
			srv = _srv
			s := newCommandServer(h, c.browser, c)
			RegisterCommandHandlerServer(srv.Registrar(), s)
		}, "text", "client", "command")
	return
}

// Edit requests editor server to edit buf.
func (c *Client) Edit(file workspaceapi.URI, buf *cell.Buffer) (textapi.Handler, error) {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	req := NewEditRequest(file, buf)

	_, err := c.ed.Edit(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}

	return Token{URI: file}, nil
}

// Editor satisfies text.Editor
func (c *Client) Editor(file workspaceapi.URI) (textapi.Handler, error) {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	req := EditorRequest{ResourceName: NewURI(file)}

	_, err := c.ed.Editor(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return nil, err
	}

	return Token{URI: file}, nil
}

// SubscribeEvents requests the editor server to subscribe sub to ev.
func (c *Client) SubscribeEvents(
	evs []textapi.EventType, h textapi.EventHandler,
) error {
	c.log(log.TraceLevel, "SubscribeEvents: %v", evs)
	stream, err := c.ed.Subscribe(c.clientCtx)
	c.log(log.TraceLevel, "SubscribeEvents: %v: %v", evs, err)
	if err != nil {
		return err
	}

	var req EditorSubscribeRequest
	for _, ev := range evs {
		req.Type = append(req.Type, protoType(textapi.Event{Type: ev}))
	}

	err = stream.Send(&req)
	c.log(log.TraceLevel, "sent initial request: %v", err)
	if err != nil {
		return fmt.Errorf("stream send request: %v", err)
	}

	handler := newEventStreamServer(c.clientCtx, stream, h)
	go handler.receiveEvents(c)

	return nil
}

// SubscribeCommandrequests the editor server to register cmd with h.
func (c *Client) SubscribeCommand(cmd string, h textapi.CommandHandler) error {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	channelID, srv, err := c.serveCommandHandler(h)
	if err != nil {
		return fmt.Errorf("serve command handler: %w", err)
	}

	req := RegisterCommandRequest{Command: cmd, ChannelId: channelID}
	_, err = c.ed.Register(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		if srv != nil {
			srv.Stop()
		}
		return err
	}

	return nil
}

func makeLocationListRequest(
	uri workspaceapi.URI, priority textapi.LocationPriority,
	listID string, l textapi.LocationList,
) SetLocationListRequest {
	req := SetLocationListRequest{
		ResourceName: NewURI(uri),
		ListId:       listID,
		Priority:     uint32(priority),
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
	h textapi.Handler, pri textapi.LocationPriority, ID string, l textapi.LocationList,
) error {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	token := h.(Token)
	req := makeLocationListRequest(token.URI, pri, ID, l)
	_, err := c.ed.SetLocationList(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

func (c *Client) moveToLocation(h textapi.Handler, ID string, next bool) (err error) {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	token := h.(Token)
	req := MoveToLocationRequest{ResourceName: NewURI(token.URI), ListId: ID}
	if next {
		_, err = c.ed.MoveToNextLocation(ctx, &req)
	} else {
		_, err = c.ed.MoveToPrevLocation(ctx, &req)
	}
	runtime.KeepAlive(c)
	return err
}

// MoveToPrevLocation requests the editor server to move cursor to the previous location
// in location list identified by ID.
func (c *Client) MoveToPrevLocation(h textapi.Handler, ID string) error {
	err := c.moveToLocation(h, ID, false)
	runtime.KeepAlive(c)
	return err
}

// MoveToNextLocation requests the editor server to move cursor to the next location
// in location list identified by ID.
func (c *Client) MoveToNextLocation(h textapi.Handler, ID string) error {
	err := c.moveToLocation(h, ID, true)
	runtime.KeepAlive(c)
	return err
}

// SetCursor requests the editor server to move cursor to pos
func (c *Client) SetCursor(h textapi.Handler, pos term.Coordinates) error {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	token := h.(Token)
	var protoPos termpb.Coordinates
	protoPos.FromModel(pos)
	req := SetCursorRequest{Pos: &protoPos, ResourceName: NewURI(token.URI)}
	_, err := c.ed.SetCursor(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// Cursor requests the editor server to move cursor to pos
func (c *Client) Cursor(h textapi.Handler) (term.Coordinates, error) {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	token := h.(Token)
	req := CursorRequest{ResourceName: NewURI(token.URI)}
	res, err := c.ed.Cursor(ctx, &req)
	runtime.KeepAlive(c)
	if err != nil {
		return term.Coordinates{}, err
	}
	return res.GetPos().ToModel(), nil
}

// CellEditor satisfies text.Editor.
func (c *Client) CellEditor(h textapi.Handler) textapi.CellEditor {
	token := h.(Token)
	return clientWriter{client: c, uri: token.URI}
}

// CellView satisfies text.Editor.
func (c *Client) CellView(h textapi.Handler) textapi.CellView {
	token := h.(Token)
	return clientView{client: c, uri: token.URI}
}

// SetDefaultAttributes satisfies text.Editor.
func (c *Client) SetDefaultAttributes(h textapi.Handler, attrs term.Attributes) error {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	token := h.(Token)
	var rpcAttrs termpb.Attributes
	rpcAttrs.FromModel(attrs)
	req := SetDefaultAttributesRequest{
		ResourceName: NewURI(token.URI),
		Attributes:   &rpcAttrs,
	}
	_, err := c.ed.SetDefaultAttributes(ctx, &req)
	runtime.KeepAlive(c)
	return err
}

// Close closes all resources associated with this client.
func (c *Client) Close() (ret error) {
	if closer, ok := c.cc.(io.Closer); ok {
		ret = closer.Close()
	}
	c.cc = nil
	if c.clientCancelCtx != nil {
		c.clientCancelCtx()
		c.clientCancelCtx = nil
	}
	runtime.SetFinalizer(c, nil)
	return ret
}

func (c *Client) ctxWithTimeout() (context.Context, func()) {
	ctx, cancel := context.WithTimeout(c.clientCtx, defaultTimeout)
	return ctx, cancel
}
