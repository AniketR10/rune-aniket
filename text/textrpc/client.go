// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package textrpc

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/browser/browserrpc"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	termrpc "unstable.build/go-tui/term/termrpc"
	"unstable.build/go-tui/text"
)

const (
	defaultTimeout = 4 * time.Second
)

var _ text.Handler = Token{}

// Token wraps a browser.Token to satisfy editor.Handler.
type Token struct {
	browserrpc.Token
	workspaceapi.URI
}

// Resource satisfies text.Handler
func (t Token) Resource() workspaceapi.URI {
	return t.URI
}

// SetWrap satisfies text.Handler
func (t Token) SetWrap(wrap bool) {
}

// SetCursorAtScroll satisfies text.Handler
func (t Token) SetCursorAtScroll(term.Coordinates) bool {
	return false
}

// ShowCommandBar satisfies text.Handler.
func (t Token) ShowCommandBar(show bool) {
}

// SeekUp satisfies text.Handler.
func (t Token) SeekUp() bool {
	return false
}

// SeekDown satisfies text.Handler.
func (t Token) SeekDown() bool {
	return false
}

// SeekOffset satisfies text.Handler.
func (t Token) SeekOffset() int {
	return 0
}

// MaxSeekOffset satisfies text.Handler.
func (t Token) MaxSeekOffset() int {
	return 0
}

var _ textapi.Editor = (*Client)(nil)

// Client satisfies text.Editor by calling a remote editor over grpc.
type Client struct {
	broker          rpc.MuxBroker
	browser         *browserrpc.Client
	cc              rpc.MuxConn
	ed              EditorClient
	clientCtx       context.Context
	clientCancelCtx func()
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(
	ctx context.Context, broker rpc.MuxBroker, cc rpc.MuxConn,
) *Client {
	ret := new(Client)
	ret.Init(ctx, broker, cc)
	runtime.SetFinalizer(ret, func(c *Client) { c.Close() })
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(
	ctx context.Context, broker rpc.MuxBroker, cc rpc.MuxConn,
) {
	c.ed = NewEditorClient(cc)
	c.cc = cc
	c.broker = broker
	ok := rpc.IsContextWithWaitGroup(ctx)
	if !ok {
		ctx = rpc.ContextWithWaitGroup(ctx, new(sync.WaitGroup))
	}
	c.browser = browserrpc.NewClient(ctx, cc)
	c.clientCtx, c.clientCancelCtx = context.WithCancel(ctx)
}

func (c *Client) log(level log.Level, msg string, args ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "text.Client").Logf(level, msg, args...)
}

func (c *Client) serveCommandHandler(h textapi.CommandHandler) (
	ret string, srv rpc.MuxServer, err error,
) {
	ctxWg := rpc.WaitGroupFromContext(c.clientCtx)
	// NOTE: there's no way to unregister from the public API, so extensions
	// cannot create more than one command handler per command.
	// If we ever add unregister to the API, we should cleanup
	// cyclical references here so the Client's runtime finalizer can
	// run correctly, in the case where clients use multiple
	// clients to register and unregister new commands.
	ret, err = rpc.AcceptAndServeChannel(c.clientCtx, c.broker,
		func(channelID string, _srv rpc.MuxServer) {
			ctxWg.Add(1)
			srv = _srv
			s := newCommandServer(h, c.browser, c)
			RegisterCommandHandlerServer(srv.Registrar(), s)
		}, "text", "client", "command")
	if err == nil {
		go func(ctx context.Context) {
			defer ctxWg.Done()
			<-ctx.Done()
			srv.Stop()
		}(c.clientCtx)
	}
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
	stream, err := c.ed.SubscribeEvent(c.clientCtx)
	c.log(log.TraceLevel, "SubscribeEvents: %v: %v", evs, err)
	if err != nil {
		return err
	}

	var req SubscribeEventRequest
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

// SubscribeCommand requests the editor server to register cmd with h.
func (c *Client) SubscribeCommand(man textapi.CommandManual, h textapi.CommandHandler) error {
	ctx, cancel := c.ctxWithTimeout()
	defer cancel()

	channelID, srv, err := c.serveCommandHandler(h)
	if err != nil {
		return fmt.Errorf("serve command handler: %w", err)
	}

	rpcMan := makeProtoManual(man)
	req := RegisterCommandRequest{Command: &rpcMan, ChannelId: channelID}
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
		var from, to termrpc.Coordinates
		var attr termrpc.Attributes
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
	return req // nolint:govet
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
	var protoPos termrpc.Coordinates
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
	var rpcAttrs termrpc.Attributes
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
	if c.clientCancelCtx != nil {
		c.clientCancelCtx()
		c.clientCancelCtx = nil
	}
	ret = c.cc.Close()
	c.cc = nil
	runtime.SetFinalizer(c, nil)
	return ret
}

func (c *Client) ctxWithTimeout() (context.Context, func()) {
	ctx, cancel := context.WithTimeout(c.clientCtx, defaultTimeout)
	return ctx, cancel
}

func makeProtoManual(man textapi.CommandManual) CommandManual {
	var cmds []*CommandManual
	for _, cmd := range man.Commands {
		childManual := new(CommandManual)
		*childManual = makeProtoManual(cmd)
		cmds = append(cmds, childManual)
	}
	ret := CommandManual{
		Name:     man.Name,
		Summary:  man.Summary,
		Synopsis: man.Synopsis,
		Commands: cmds,
	}
	return ret // nolint:govet
}
