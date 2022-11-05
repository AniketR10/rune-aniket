package rpc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	
	"unstable.build/go-tui/workspace"
)

type executorServer struct {
	wp workspace.API
	e  workspace.Executor

	mu            sync.Locker
	resources     map[int32]executorResource
	nextHandlerID int32
}

func (s *executorServer) Command(ctx context.Context, req *CommandRequest) (
	*CommandResponse, error,
) {
	name := req.GetName()
	args := req.GetArgs()
	pid, err := s.e.Command(name, args...)
	if err != nil {
		return nil, err
	}
	resp := new(CommandResponse)
	resp.Pid = int32(pid)
	return resp, nil
}

func (s *executorServer) Start(ctx context.Context, req *StartRequest) (
	*StartResponse, error,
) {
	pid := req.GetPid()
	err := s.e.Start(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StartResponse)
	return resp, nil
}

func removePidResources(
	mu sync.Locker, res map[int32]executorResource, pid workspace.Pid,
) []executorResource {
	mu.Lock()
	defer mu.Unlock()

	var ids []int32
	for handlerID, res := range res {
		if res.pid == pid {
			ids = append(ids, handlerID)
			_ = res.closer.Close()
		}
	}

	var ret []executorResource
	for _, id := range ids {
		ret = append(ret, res[id])
		delete(res, id)
	}
	return ret
}

func (s *executorServer) removePidResources(pid workspace.Pid) {
	res := removePidResources(s.mu, s.resources, pid)
	s.log(log.TraceLevel, "cleaned all resources of pid %d: %#v", pid, res)
}

func (s *executorServer) Wait(ctx context.Context, req *WaitRequest) (
	*WaitResponse, error,
) {
	pid := req.GetPid()
	s.log(log.TraceLevel, "Wait(pid=%d)", pid)
	defer s.removePidResources(workspace.Pid(pid))
	err := s.e.Wait(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(WaitResponse)
	return resp, nil
}

func (s *executorServer) Signal(ctx context.Context, req *SignalRequest) (
	*SignalResponse, error,
) {
	pid := req.GetPid()
	signal := req.GetSig()
	err := s.e.Signal(workspace.Pid(pid), syscall.Signal(signal))
	if err != nil {
		return nil, err
	}
	resp := new(SignalResponse)
	return resp, nil
}

func (s *executorServer) StderrPipe(ctx context.Context, req *StdioPipeRequest) (
	*StdioPipeResponse, error,
) {
	pid := req.GetPid()
	pipe, err := s.e.StderrPipe(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StdioPipeResponse)
	handlerID := s.addHandle(workspace.Pid(pid), &syncReader{reader: pipe})
	resp.HandlerId = handlerID
	return resp, nil
}

func (s *executorServer) StdoutPipe(ctx context.Context, req *StdioPipeRequest) (
	*StdioPipeResponse, error,
) {
	pid := req.GetPid()
	pipe, err := s.e.StdoutPipe(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StdioPipeResponse)
	handlerID := s.addHandle(workspace.Pid(pid), &syncReader{reader: pipe})
	resp.HandlerId = handlerID
	return resp, nil
}

func (s *executorServer) StdinPipe(ctx context.Context, req *StdioPipeRequest) (
	*StdioPipeResponse, error,
) {
	pid := req.GetPid()
	pipe, err := s.e.StdinPipe(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StdioPipeResponse)
	handlerID := s.addHandle(workspace.Pid(pid), &syncWriter{writer: pipe})
	resp.HandlerId = handlerID
	return resp, nil
}

// Read satisfies SchemeServer.
func (s *executorServer) Read(ctx context.Context, req *ReadRequest) (
	*ReadResponse, error,
) {
	f, ok := s.getReader(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	buf := make([]byte, req.GetN())
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read error: %s", err)
	}
	resp := new(ReadResponse)
	resp.Data = buf[:n]
	resp.N = int64(n)
	resp.IsEof = err == io.EOF
	return resp, nil
}

// Write satisfies SchemeServer.
func (s *executorServer) Write(ctx context.Context, req *WriteRequest) (
	*WriteResponse, error,
) {
	f, ok := s.getWriter(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	n, err := f.Write([]byte(req.GetData()))
	if err != nil {
		return nil, fmt.Errorf("write error: %s", err)
	}
	resp := new(WriteResponse)
	resp.N = int64(n)
	return resp, nil
}

// Close satisfies SchemeServer.
func (s *executorServer) Close(ctx context.Context, req *CloseFileRequest) (
	*CloseFileResponse, error,
) {
	res, ok := s.resources[req.GetHandlerId()]
	if !ok {
		return nil, errFileNotOpen
	}
	defer s.removeHandle(req.GetHandlerId())
	s.log(log.DebugLevel, "Close called on resource with handlerID: %d", req.GetHandlerId())
	err := res.closer.Close()
	if err != nil {
		return nil, fmt.Errorf("close error: %s", err)
	}
	return new(CloseFileResponse), nil
}

func (s *executorServer) init(e workspace.Executor, locker sync.Locker) {
	s.resources = make(map[int32]executorResource)
	s.nextHandlerID = 0
	s.mu = locker
	s.e = e
}

func (s *executorServer) log(
	level log.Level, msg string, args ...interface{},
) {
	log.
		WithField(logging.KeyClass, "executorServer").
		Logf(level, msg, args...)
}

func (s *executorServer) addHandle(pid workspace.Pid, closer io.Closer) int32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextHandlerID++
	res := executorResource{closer: closer, pid: pid}
	s.resources[s.nextHandlerID] = res
	s.log(log.TraceLevel, "added resource %#v with handlerID %d",
		res, s.nextHandlerID)
	return s.nextHandlerID
}

func (s *executorServer) removeHandle(handlerID int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, _ := s.resources[handlerID]
	s.log(log.TraceLevel, "removing resource %#v with handlerID %d",
		res, handlerID)
	delete(s.resources, handlerID)
}

func (s *executorServer) getWriter(handlerID int32) (io.WriteCloser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.resources[handlerID]
	if !ok {
		return nil, false
	}
	p, ok := h.closer.(io.WriteCloser)
	return p, ok
}

func (s *executorServer) getReader(handlerID int32) (io.ReadCloser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.resources[handlerID]
	if !ok {
		return nil, false
	}
	p, ok := h.closer.(io.ReadCloser)
	return p, ok
}

func (s *executorServer) getFile(handlerID int32) (workspace.File, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.resources[handlerID]
	if !ok {
		return nil, false
	}
	f, ok := h.closer.(workspace.File)
	return f, ok
}

func (s *executorServer) stop() (ret error) {
	for _, h := range s.resources {
		if err := h.closer.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	s.resources = nil

	s.log(log.TraceLevel, "stop")
	return
}

// sync over individual handles over locking entire executorServer
type syncReader struct {
	reader io.ReadCloser
	mu     sync.Mutex
}

type syncWriter struct {
	writer io.WriteCloser
	mu     sync.Mutex
}

func (r *syncReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reader.Read(p)
}

func (w *syncWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func (r *syncReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reader.Close()
}

func (w *syncWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Close()
}
