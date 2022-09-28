package rpc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"unstable.build/go-tui/workspace"
)

var _ SchemeServer = (*SchemeServerImpl)(nil)

// SchemeServerImpl exposes a workspace.Scheme over the wire and
// satisfies SchemeServer grpc interface.
type SchemeServerImpl struct {
	UnimplementedSchemeServer
	executorServer

	scheme workspace.Scheme
}

// NewSchemeServer allocates storage for a new SchemeServerImpl and initializes it
// with the given scheme.
func NewSchemeServer(scheme workspace.Scheme) *SchemeServerImpl {
	ret := new(SchemeServerImpl)
	ret.Init(scheme)
	return ret
}

// Init initializes this SchemeServerImpl with the given scheme.
func (s *SchemeServerImpl) Init(scheme workspace.Scheme) {
	s.executorServer.init(scheme)
	s.scheme = scheme
}

// Open satisfies SchemeServer.
func (s *SchemeServerImpl) Open(ctx context.Context, req *OpenRequest) (
	*OpenResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	f, err := s.scheme.Open(filename, int(req.GetFlag()), os.FileMode(req.GetMode()))
	if err != nil {
		if err.IsExist || err.IsNotExist || err.IsPermission {
			resp := new(OpenResponse)
			resp.IsExistErr = err.IsExist
			resp.IsNotExistErr = err.IsNotExist
			resp.IsPermissionErr = err.IsPermission
			return resp, nil
		}
		return nil, fmt.Errorf("open %s error: %s", filename, err)
	}

	handlerID := s.addHandle(&syncFile{file: f})
	resp := new(OpenResponse)
	resp.HandlerId = handlerID
	return resp, nil
}

// Remove satisfies SchemeServer.
func (s *SchemeServerImpl) Remove(ctx context.Context, req *RemoveRequest) (
	*RemoveResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	err := s.scheme.Remove(filename)
	if err != nil {
		return nil, fmt.Errorf("remove %s error: %s", filename, err)
	}
	return new(RemoveResponse), nil
}

// Rename satisfies SchemeServer.
func (s *SchemeServerImpl) Rename(ctx context.Context, req *RenameRequest) (
	*RenameResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	err := s.scheme.Rename(filename, req.GetNewfilename())
	if err != nil {
		return nil, fmt.Errorf("rename %s error: %s", filename, err)
	}
	return new(RenameResponse), nil
}

// Stat satisfies SchemeServer.
func (s *SchemeServerImpl) Stat(ctx context.Context, req *StatRequest) (
	*StatResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	var fs os.FileInfo
	if req.GetLstat() {
		fs, err = s.scheme.Lstat(req.GetFilename())
	} else {
		fs, err = s.scheme.Stat(req.GetFilename())
	}
	if err != nil {
		return nil, fmt.Errorf("stat %s error: %s", req.GetFilename(), err)
	}
	resp := new(StatResponse)
	resp.Name = fs.Name()
	resp.Size = fs.Size()
	resp.Mode = int32(fs.Mode())
	resp.ModTime = new(timestamppb.Timestamp)
	*resp.ModTime = stdTimeToProto(fs.ModTime())
	resp.IsDir = fs.IsDir()

	return resp, nil
}

// ReadLink satisfies SchemeServer.
func (s *SchemeServerImpl) ReadLink(ctx context.Context, req *ReadLinkRequest) (
	*ReadLinkResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	fil, err := s.scheme.ReadLink(filename)
	if err != nil {
		return nil, fmt.Errorf("read link %s error: %s", filename, err)
	}
	resp := new(ReadLinkResponse)
	resp.Filename = fil
	return resp, nil
}

// Command satisfies SchemeServer
func (s *SchemeServerImpl) Command(
	ctx context.Context, req *CommandRequest,
) (*CommandResponse, error) {
	return s.executorServer.Command(ctx, req)
}

// Start satisfies SchemeServer
func (s *SchemeServerImpl) Start(ctx context.Context, req *StartRequest) (*StartResponse, error) {
	return s.executorServer.Start(ctx, req)
}

// Wait satisfies SchemeServer
func (s *SchemeServerImpl) Wait(ctx context.Context, req *WaitRequest) (*WaitResponse, error) {
	return s.executorServer.Wait(ctx, req)
}

// Signal satisfies SchemeServer
func (s *SchemeServerImpl) Signal(ctx context.Context, req *SignalRequest) (*SignalResponse, error) {
	return s.executorServer.Signal(ctx, req)
}

// StderrPipe satisfies SchemeServer
func (s *SchemeServerImpl) StderrPipe(ctx context.Context, req *StdioPipeRequest) (*StdioPipeResponse, error) {
	return s.executorServer.StderrPipe(ctx, req)
}

// StdoutPipe satisfies SchemeServer
func (s *SchemeServerImpl) StdoutPipe(ctx context.Context, req *StdioPipeRequest) (*StdioPipeResponse, error) {
	return s.executorServer.StdoutPipe(ctx, req)
}

// StdinPipe satisfies SchemeServer
func (s *SchemeServerImpl) StdinPipe(ctx context.Context, req *StdioPipeRequest) (*StdioPipeResponse, error) {
	return s.executorServer.StdinPipe(ctx, req)
}

// Close satisfies SchemeServer
func (s *SchemeServerImpl) Close(ctx context.Context, req *CloseFileRequest) (*CloseFileResponse, error) {
	return s.executorServer.Close(ctx, req)
}

// Read satisfies SchemeServer
func (s *SchemeServerImpl) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	return s.executorServer.Read(ctx, req)
}

// Write satisfies SchemeServer
func (s *SchemeServerImpl) Write(ctx context.Context, req *WriteRequest) (*WriteResponse, error) {
	return s.executorServer.Write(ctx, req)
}

// Stop closes all associated resources with this SchemeServerImpl.
func (s *SchemeServerImpl) Stop() {
	s.executorServer.stop()
}

// Sync satisfies SchemeServer.
func (s *SchemeServerImpl) Sync(ctx context.Context, req *SyncRequest) (
	*SyncResponse, error,
) {
	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	err := f.Sync()
	if err != nil {
		return nil, fmt.Errorf("sync error: %s", err)
	}
	return new(SyncResponse), nil
}

// Truncate satisfies SchemeServer.
func (s *SchemeServerImpl) Truncate(ctx context.Context, req *TruncateRequest) (
	*TruncateResponse, error,
) {
	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	err := f.Truncate(req.GetSize())
	if err != nil {
		return nil, fmt.Errorf("truncate error: %s", err)
	}
	return new(TruncateResponse), nil
}

// Seek satisfies SchemeServer.
func (s *SchemeServerImpl) Seek(ctx context.Context, req *SeekRequest) (
	*SeekResponse, error,
) {
	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	newOffset, err := f.Seek(req.GetOffset(), int(req.GetWhence()))
	if err != nil {
		return nil, fmt.Errorf("seek error: %s", err)
	}
	resp := new(SeekResponse)
	resp.NewOffset = newOffset
	return resp, nil
}

func stdTimeToProto(ts time.Time) timestamppb.Timestamp {
	seconds := ts.Unix()
	nanos := ts.Nanosecond()
	return timestamppb.Timestamp{Seconds: seconds, Nanos: int32(nanos)}
}

// sync workspace.File
type syncFile struct {
	mu   sync.Mutex
	file workspace.File
}

func (r *syncFile) Name() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Name()
}

func (r *syncFile) Stat() (os.FileInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Stat()
}

func (r *syncFile) Sync() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Sync()
}

func (r *syncFile) Truncate(size int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Truncate(size)
}

func (r *syncFile) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Seek(offset, whence)
}

func (r *syncFile) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Read(p)
}

func (w *syncFile) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Write(p)
}

func (r *syncFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}
