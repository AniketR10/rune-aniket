package rpc

import (
	"context"
	"errors"
	"sync"

	"unstable.build/go-tui/workspace"
)

var errFileNotOpen = errors.New("file is not open")
var _ WorkspaceServer = (*Server)(nil)

// Server is a workspace server implementation which processes one request at a time.
type Server struct {
	UnimplementedWorkspaceServer
	sharedRPCImpl

	wp workspace.Workspace
}

// NewServer allocates storage for a new server and initializes it with wp.
func NewServer(wp workspace.Workspace, locker sync.Locker) *Server {
	ret := new(Server)
	ret.Init(wp, locker)
	return ret
}

// Init initializes this Server with the given workspace
func (s *Server) Init(wp workspace.Workspace, locker sync.Locker) {
	s.sharedRPCImpl.executorServer.init(wp, locker)
	s.sharedRPCImpl.scheme = wp
	s.wp = wp
}

// URI satisfies WorkspaceServer
func (s *Server) URI(ctx context.Context, req *URIRequest) (
	*URIResponse, error,
) {
	uri, err := s.wp.URI(req.GetPath())
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

// NewPty satisfies WorkspaceServer.
func (s *Server) NewPty(ctx context.Context, req *NewPtyRequest) (
	*NewPtyResponse, error,
) {
	return s.sharedRPCImpl.NewPty(ctx, req)
}

// SetPtySize satisfies WorkspaceServer.
func (s *Server) SetPtySize(ctx context.Context, req *SetPtySizeRequest) (
	*SetPtySizeResponse, error,
) {
	return s.sharedRPCImpl.SetPtySize(ctx, req)
}

// Open satisfies WorkspaceServer.
func (s *Server) Open(ctx context.Context, req *OpenRequest) (
	*OpenResponse, error,
) {
	return s.sharedRPCImpl.Open(ctx, req)
}

// Remove satisfies WorkspaceServer.
func (s *Server) Remove(ctx context.Context, req *RemoveRequest) (
	*RemoveResponse, error,
) {
	return s.sharedRPCImpl.Remove(ctx, req)
}

// Stat satisfies SchemeServer.
func (s *Server) Stat(ctx context.Context, req *StatRequest) (
	*StatResponse, error,
) {
	return s.sharedRPCImpl.Stat(ctx, req)
}

// ReadDir satisfies SchemeServer.
func (s *Server) ReadDir(ctx context.Context, req *ReadDirRequest) (
	*ReadDirResponse, error,
) {
	return s.sharedRPCImpl.ReadDir(ctx, req)
}

// Stop closes all resources associated with this server.
func (s *Server) Stop() error {
	return s.executorServer.stop()
}
