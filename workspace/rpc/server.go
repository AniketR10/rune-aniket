package rpc

import (
	"context"
	"errors"

	"github.com/ernestrc/go-tui/workspace"
)

var errFileNotOpen = errors.New("file is not open")
var _ WorkspaceServer = (*Server)(nil)

// Server is a workspace server implementation which processes one request at a time.
type Server struct {
	UnimplementedWorkspaceServer
	executorServer

	wp workspace.API
}

// NewServer allocates storage for a new server and initializes it with wp.
func NewServer(wp workspace.Workspace) *Server {
	ret := new(Server)
	ret.Init(wp)
	return ret
}

// Init initializes this Server with the given workspace
func (s *Server) Init(wp workspace.Workspace) {
	s.executorServer.init(wp)
	s.wp = wp
}

// URI satisfies WorkspaceServer
func (s *Server) URI(ctx context.Context, req *URIRequest) (
	*URIResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uri, err := s.wp.URI(req.GetPath())
	if err != nil {
		return nil, err
	}
	resp := new(URIResponse)
	resp.Uri = uri.String()
	return resp, nil
}

// Getwd satisfies WorkspaceServer
func (s *Server) Getwd(ctx context.Context, req *GetwdRequest) (
	*URIResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uri, err := s.wp.Getwd()
	if err != nil {
		return nil, err
	}
	resp := new(URIResponse)
	resp.Uri = uri.String()
	return resp, nil
}

// Command satisfies WorkspaceServer
func (s *Server) Command(
	ctx context.Context, req *CommandRequest,
) (*CommandResponse, error) {
	return s.executorServer.Command(ctx, req)
}

// Start satisfies WorkspaceServer
func (s *Server) Start(ctx context.Context, req *StartRequest) (*StartResponse, error) {
	return s.executorServer.Start(ctx, req)
}

// Wait satisfies WorkspaceServer
func (s *Server) Wait(ctx context.Context, req *WaitRequest) (*WaitResponse, error) {
	return s.executorServer.Wait(ctx, req)
}

// Signal satisfies WorkspaceServer
func (s *Server) Signal(ctx context.Context, req *SignalRequest) (*SignalResponse, error) {
	return s.executorServer.Signal(ctx, req)
}

// StderrPipe satisfies WorkspaceServer
func (s *Server) StderrPipe(ctx context.Context, req *StdioPipeRequest) (*StdioPipeResponse, error) {
	return s.executorServer.StderrPipe(ctx, req)
}

// StdoutPipe satisfies WorkspaceServer
func (s *Server) StdoutPipe(ctx context.Context, req *StdioPipeRequest) (*StdioPipeResponse, error) {
	return s.executorServer.StdoutPipe(ctx, req)
}

// StdinPipe satisfies WorkspaceServer
func (s *Server) StdinPipe(ctx context.Context, req *StdioPipeRequest) (*StdioPipeResponse, error) {
	return s.executorServer.StdinPipe(ctx, req)
}

// Close satisfies WorkspaceServer
func (s *Server) Close(ctx context.Context, req *CloseFileRequest) (*CloseFileResponse, error) {
	return s.executorServer.Close(ctx, req)
}

// Read satisfies WorkspaceServer
func (s *Server) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	return s.executorServer.Read(ctx, req)
}

// Write satisfies WorkspaceServer
func (s *Server) Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error) {
	return s.executorServer.Write(ctx, req)
}

// Stop closes all resources associated with this server.
func (s *Server) Stop() {
	s.executorServer.stop()
}
