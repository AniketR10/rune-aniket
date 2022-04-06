package workspace

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	workspacepb "github.com/ernestrc/go-tui/workspace/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var errFileNotOpen = errors.New("file is not open")
var _ workspacepb.WorkspaceServer = (*Server)(nil)

// Server is a workspace server implementation which processes one request at a time.
type Server struct {
	workspacepb.UnimplementedWorkspaceServer
	openFunc     openFunc
	removeFunc   removeFunc
	renameFunc   renameFunc
	statFunc     statFunc
	lstatFunc    statFunc
	readLinkFunc readLinkFunc

	mu            sync.Mutex
	handles       map[int32]osFile
	nextHandlerID int32
}

func NewServer() *Server {
	ret := new(Server)
	ret.Init()
	return ret
}

func (s *Server) Init() {
	s.handles = make(map[int32]osFile)
	s.nextHandlerID = 0
	s.openFunc = osOpenFileFunc()
	s.removeFunc = os.Remove
	s.renameFunc = os.Rename
	s.statFunc = os.Stat
	s.lstatFunc = os.Lstat
}

func (s *Server) getFile(handlerID int32) (osFile, bool) {
	f, ok := s.handles[handlerID]
	return f, ok
}

func (s *Server) removeFile(handlerID int32) {
	delete(s.handles, handlerID)
}

func (s *Server) addFile(f osFile) int32 {
	s.nextHandlerID++
	s.handles[s.nextHandlerID] = f
	return s.nextHandlerID
}

func (s *Server) Open(ctx context.Context, req *workspacepb.OpenRequest) (
	*workspacepb.OpenResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	f, err := s.openFunc(filename, int(req.GetFlag()), os.FileMode(req.GetMode()))
	if err != nil {
		if err.isExist || err.isNotExist || err.isPermission {
			resp := new(workspacepb.OpenResponse)
			resp.IsExistErr = err.isExist
			resp.IsNotExistErr = err.isNotExist
			resp.IsPermissionErr = err.isPermission
			return resp, nil
		}
		return nil, fmt.Errorf("open %s error: %s", filename, err)
	}
	handlerID := s.addFile(f)
	resp := new(workspacepb.OpenResponse)
	resp.HandlerId = handlerID
	return resp, nil
}

func (s *Server) Remove(ctx context.Context, req *workspacepb.RemoveRequest) (
	*workspacepb.RemoveResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	err := s.removeFunc(filename)
	if err != nil {
		return nil, fmt.Errorf("remove %s error: %s", filename, err)
	}
	return new(workspacepb.RemoveResponse), nil
}

func (s *Server) Rename(ctx context.Context, req *workspacepb.RenameRequest) (
	*workspacepb.RenameResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	err := s.renameFunc(filename, req.GetNewfilename())
	if err != nil {
		return nil, fmt.Errorf("rename %s error: %s", filename, err)
	}
	return new(workspacepb.RenameResponse), nil
}

func (s *Server) Stat(ctx context.Context, req *workspacepb.StatRequest) (
	*workspacepb.StatResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	var fs os.FileInfo
	if req.GetLstat() {
		fs, err = s.lstatFunc(req.GetFilename())
	} else {
		fs, err = s.statFunc(req.GetFilename())
	}
	if err != nil {
		return nil, fmt.Errorf("stat %s error: %s", req.GetFilename(), err)
	}
	resp := new(workspacepb.StatResponse)
	resp.Name = fs.Name()
	resp.Size = fs.Size()
	resp.Mode = int32(fs.Mode())
	resp.ModTime = new(timestamppb.Timestamp)
	*resp.ModTime = stdTimeToProto(fs.ModTime())
	resp.IsDir = fs.IsDir()

	return resp, nil
}

func (s *Server) Sync(ctx context.Context, req *workspacepb.SyncRequest) (
	*workspacepb.SyncResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	err := f.Sync()
	if err != nil {
		return nil, fmt.Errorf("sync error: %s", err)
	}
	return new(workspacepb.SyncResponse), nil
}

func (s *Server) Truncate(ctx context.Context, req *workspacepb.TruncateRequest) (
	*workspacepb.TruncateResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	err := f.Truncate(req.GetSize())
	if err != nil {
		return nil, fmt.Errorf("truncate error: %s", err)
	}
	return new(workspacepb.TruncateResponse), nil
}

func (s *Server) Close(ctx context.Context, req *workspacepb.CloseFileRequest) (
	*workspacepb.CloseFileResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	defer s.removeFile(req.GetHandlerId())
	err := f.Close()
	if err != nil {
		return nil, fmt.Errorf("close error: %s", err)
	}
	return new(workspacepb.CloseFileResponse), nil
}

func (s *Server) Seek(ctx context.Context, req *workspacepb.SeekRequest) (
	*workspacepb.SeekResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	newOffset, err := f.Seek(req.GetOffset(), int(req.GetWhence()))
	if err != nil {
		return nil, fmt.Errorf("seek error: %s", err)
	}
	resp := new(workspacepb.SeekResponse)
	resp.NewOffset = newOffset
	return resp, nil
}

func (s *Server) Read(ctx context.Context, req *workspacepb.ReadRequest) (
	*workspacepb.ReadResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	buf := make([]byte, req.GetN())
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read error: %s", err)
	}
	resp := new(workspacepb.ReadResponse)
	resp.Data = string(buf[:n])
	resp.N = int64(n)
	resp.IsEof = err == io.EOF
	return resp, nil
}

func (s *Server) Write(ctx context.Context, req *workspacepb.WriteRequest) (
	*workspacepb.WriteResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	n, err := f.Write([]byte(req.GetData()))
	if err != nil {
		return nil, fmt.Errorf("write error: %s", err)
	}
	resp := new(workspacepb.WriteResponse)
	resp.N = int64(n)
	return resp, nil
}

func (s *Server) ReadLink(ctx context.Context, req *workspacepb.ReadLinkRequest) (
	*workspacepb.ReadLinkResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := req.GetFilename()
	fil, err := s.readLinkFunc(filename)
	if err != nil {
		return nil, fmt.Errorf("read link %s error: %s", filename, err)
	}
	resp := new(workspacepb.ReadLinkResponse)
	resp.Filename = fil
	return resp, nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, h := range s.handles {
		_ = h.Close()
	}
	s.handles = nil
}

func stdTimeToProto(ts time.Time) timestamppb.Timestamp {
	seconds := ts.Unix()
	nanos := ts.Nanosecond()
	return timestamppb.Timestamp{Seconds: seconds, Nanos: int32(nanos)}
}
