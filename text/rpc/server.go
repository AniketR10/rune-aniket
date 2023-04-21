package rpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/proto"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
)

var (
	errHandlerNotFound = errors.New("handler not found")
)

// Server serves an Editor over GRPC.
type Server struct {
	UnimplementedEditorServer

	broker proto.MuxBroker

	editor struct {
		text.Editor
		sync.Locker
	}

	// subscriptions, just to unsubscribe upon close
	eventSub []text.EventHandler
	cmdSub   []string
}

// NewServer allocates storage for a new Server and initializes it.
func NewServer(
	broker proto.MuxBroker, editor text.Editor, lock sync.Locker,
) *Server {
	ret := new(Server)
	ret.Init(broker, editor, lock)
	return ret
}

// Init initializes this Server with broker and browser.
func (s *Server) Init(
	broker proto.MuxBroker, editor text.Editor, lock sync.Locker,
) {
	s.broker = broker
	s.editor.Editor = editor
	s.editor.Locker = lock
}

func (s *Server) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "text.Server").Logf(level, msg, args...)
}

func (s *Server) dialCommandHandler(channelID string) (text.CommandHandler, error) {
	s.log(log.TraceLevel,
		"(%p editor.Server): dialing command handler with id: %s", s, channelID)
	handlerConn, err := s.broker.DialChannel(channelID, os.Args[0], "textpb.Server")
	if err != nil {
		return nil, err
	}
	return newCommandClient(handlerConn, s), nil
}

func (s *Server) editHandler(
	resource workspaceapi.URI, get func(workspaceapi.URI) (text.Handler, error),
) error {
	s.editor.Lock()
	defer s.editor.Unlock()

	_, err := get(resource)
	if err != nil {
		return err
	}

	return nil
}

// Edit satisfies EditorServer
func (s *Server) Edit(ctx context.Context, in *EditRequest) (
	*EditResponse, error,
) {
	uri, err := NewURIFromProto(in.GetResourceName())
	if err != nil {
		return nil, err
	}

	err = s.editHandler(uri,
		func(resource workspaceapi.URI) (text.Handler, error) {
			buf := EditRequestToBuffer(in)
			return s.editor.Edit(uri, buf)
		})
	if err != nil {
		return nil, err
	}

	return &EditResponse{}, nil
}

// Editor satisfies EditorServer
func (s *Server) Editor(ctx context.Context, in *EditorRequest) (
	*EditorResponse, error,
) {
	uri, err := NewURIFromProto(in.GetResourceName())
	if err != nil {
		return nil, err
	}

	err = s.editHandler(uri,
		func(resource workspaceapi.URI) (text.Handler, error) {
			return s.editor.Editor.Editor(uri)
		})
	if err != nil {
		return nil, err
	}

	return &EditorResponse{}, nil
}

// Subscribe satisfies EditorServer
func (s *Server) Subscribe(stream Editor_SubscribeServer) error {
	req, err := stream.Recv()
	s.log(log.TraceLevel, "Subscribe: received request: %v: %v", req.GetType(), err)
	if err != nil {
		return fmt.Errorf("receive stream request: %v", err)
	}

	var evTypes []textapi.EventType
	for _, ev := range req.GetType() {
		evType, err := protoTypeToModel(ev)
		if err != nil {
			return err
		}
		evTypes = append(evTypes, evType)
	}

	handler := newEventStreamClient(stream, s.editor)

	s.editor.Lock()
	err = s.editor.SubscribeEvents(evTypes, handler)
	if err != nil {
		s.editor.Unlock()
		return fmt.Errorf("subscribe editor events: %v", err)
	}
	s.eventSub = append(s.eventSub, handler)
	s.editor.Unlock()

	s.log(log.TraceLevel, "waiting for unsubscribe")
	return handler.waitForUnsubscribe()
}

// Register satisfies EditorServer
func (s *Server) Register(ctx context.Context, in *RegisterCommandRequest) (
	*RegisterCommandResponse, error,
) {
	channelID := in.GetChannelId()
	commander, err := s.dialCommandHandler(channelID)
	if err != nil {
		return nil, err
	}

	cmd := in.GetCommand()

	s.editor.Lock()
	err = s.editor.SubscribeCommand(cmd, commander)
	if err != nil {
		s.editor.Unlock()
		return nil, err
	}
	s.cmdSub = append(s.cmdSub, cmd)
	s.editor.Unlock()

	return new(RegisterCommandResponse), nil
}

func getLocations(locs []*SetLocationListRequest_Location) (ret []textapi.Location) {
	for _, loc := range locs {
		ret = append(ret, textapi.Location{
			Attr:    loc.GetAttr().ToModel(),
			From:    loc.GetFrom().ToModel(),
			To:      loc.GetTo().ToModel(),
			Message: loc.GetMsg(),
		})
	}
	return
}

// SetLocationList satisfies EditorServer
func (s *Server) SetLocationList(ctx context.Context, in *SetLocationListRequest) (
	*SetLocationListResponse, error,
) {
	locs := in.GetLocations()
	id := in.GetListId()
	pri := in.GetPriority()

	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("SetLocationList", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	err := s.editor.SetLocationList(h, textapi.LocationPriority(pri),
		id, text.LocationSlice(getLocations(locs)))
	if err != nil {
		return nil, err
	}

	return new(SetLocationListResponse), nil
}

// MoveToNextLocation satisfies EditorServer
func (s *Server) MoveToNextLocation(ctx context.Context, in *MoveToLocationRequest) (
	res *MoveToLocationResponse, err error,
) {
	return s.moveToLocation(ctx, in, true)
}

// MoveToPrevLocation satisfies EditorServer
func (s *Server) MoveToPrevLocation(ctx context.Context, in *MoveToLocationRequest) (
	res *MoveToLocationResponse, err error,
) {
	return s.moveToLocation(ctx, in, false)
}

// SetDefaultAttributes satisfies EditorServer
func (s *Server) SetDefaultAttributes(ctx context.Context, in *SetDefaultAttributesRequest) (
	*SetDefaultAttributesResponse, error,
) {
	attrs := in.GetAttributes()

	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("SetDefaultAttributes", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	err := s.editor.SetDefaultAttributes(h, attrs.ToModel())
	if err != nil {
		return nil, err
	}

	return new(SetDefaultAttributesResponse), nil
}

// SetCursor satisfies EditorServer
func (s *Server) SetCursor(ctx context.Context, in *SetCursorRequest) (
	*SetCursorResponse, error,
) {
	pos := in.GetPos()

	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("SetCursor", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	err := s.editor.SetCursor(h, pos.ToModel())
	if err != nil {
		return nil, err
	}

	return new(SetCursorResponse), nil
}

// Cursor satisfies EditorServer
func (s *Server) Cursor(ctx context.Context, in *CursorRequest) (
	*CursorResponse, error,
) {
	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("Cursor", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	pos, err := s.editor.Cursor(h)
	if err != nil {
		return nil, err
	}

	var protoPos termpb.Coordinates
	protoPos.FromModel(pos)

	return &CursorResponse{Pos: &protoPos}, nil
}

func (s *Server) moveToLocation(
	ctx context.Context, in *MoveToLocationRequest, next bool,
) (res *MoveToLocationResponse, err error) {
	id := in.GetListId()

	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("moveToLocation", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	if next {
		err = s.editor.MoveToNextLocation(h, id)
	} else {
		err = s.editor.MoveToPrevLocation(h, id)
	}

	if err != nil {
		return nil, err
	}

	res = new(MoveToLocationResponse)
	return res, nil
}

// EditCell satisfies EditorServer
func (s *Server) EditCell(ctx context.Context, in *EditCellRequest) (
	*EditCellResponse, error,
) {
	start := in.GetStart().ToModel()
	end := in.GetEnd().ToModel()
	str := in.GetStr()

	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("Edit", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	from, to, old, err := s.editor.CellEditor(h).Edit(start, end, str)
	if err != nil {
		return nil, err
	}

	var protoFrom, protoTo termpb.Coordinates
	protoFrom.FromModel(from)
	protoTo.FromModel(to)

	res := &EditCellResponse{
		From: &protoFrom,
		To:   &protoTo,
		Old:  old,
	}
	return res, nil
}

// RawCells satisfies EditorServer
func (s *Server) RawCells(ctx context.Context, in *RawCellsRequest) (
	*RawCellsResponse, error,
) {
	s.editor.Lock()
	defer s.editor.Unlock()

	h, ok := s.getHandler("RawCells", in.GetResourceName())
	if !ok {
		return nil, errHandlerNotFound
	}

	cells, err := s.editor.CellView(h).RawCells()
	if err != nil {
		return nil, err
	}

	return NewRawCellsResponse(cells), nil
}

// Close closes all resources associated with this server.
func (s *Server) Close() (err error) {
	// ensure that we unsubscribe all subscribers created
	// by this server. Some of these might already been unsubscribed,
	// so this completes the cleanup for the ones that haven't.
	for _, sub := range s.cmdSub {
		_ = s.editor.UnsubscribeCommand(sub)
	}
	for _, sub := range s.eventSub {
		_, _ = s.editor.UnsubscribeEvents(sub)
	}
	s.eventSub = nil
	return nil
}

func (s *Server) getHandler(call string, uri *URI) (text.Handler, bool) {
	muri, err := NewURIFromProto(uri)
	if err != nil {
		return nil, false
	}
	h, err := s.editor.Editor.Editor(muri)
	s.log(log.TraceLevel,
		"(%p editor.Server): %s: get handler with uri %s: %p %v",
		s, call, uri, h, err)
	return h, err == nil
}
