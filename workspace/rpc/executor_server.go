package rpc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"syscall"

	"unstable.build/go-tui/workspace"
)

type executorServer struct {
	wp workspace.API
	e  workspace.Executor

	mu            sync.Mutex
	handles       map[int32]io.Closer
	nextHandlerID int32
}

func (s *executorServer) Command(ctx context.Context, req *CommandRequest) (
	*CommandResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	pid := req.GetPid()
	err := s.e.Start(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StartResponse)
	return resp, nil
}

func (s *executorServer) Wait(ctx context.Context, req *WaitRequest) (
	*WaitResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pid := req.GetPid()
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
	s.mu.Lock()
	defer s.mu.Unlock()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	pid := req.GetPid()
	pipe, err := s.e.StderrPipe(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StdioPipeResponse)
	handlerID := s.addHandle(&syncReader{reader: pipe})
	resp.HandlerId = handlerID
	return resp, nil
}

func (s *executorServer) StdoutPipe(ctx context.Context, req *StdioPipeRequest) (
	*StdioPipeResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pid := req.GetPid()
	pipe, err := s.e.StdoutPipe(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StdioPipeResponse)
	handlerID := s.addHandle(&syncReader{reader: pipe})
	resp.HandlerId = handlerID
	return resp, nil
}

func (s *executorServer) StdinPipe(ctx context.Context, req *StdioPipeRequest) (
	*StdioPipeResponse, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pid := req.GetPid()
	pipe, err := s.e.StdinPipe(workspace.Pid(pid))
	if err != nil {
		return nil, err
	}
	resp := new(StdioPipeResponse)
	handlerID := s.addHandle(&syncWriter{writer: pipe})
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
	resp.Data = string(buf[:n])
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
	f, ok := s.getFile(req.GetHandlerId())
	if !ok {
		return nil, errFileNotOpen
	}
	defer s.removeHandle(req.GetHandlerId())
	err := f.Close()
	if err != nil {
		return nil, fmt.Errorf("close error: %s", err)
	}
	return new(CloseFileResponse), nil
}

func (s *executorServer) init(e workspace.Executor) {
	s.handles = make(map[int32]io.Closer)
	s.nextHandlerID = 0
	s.e = e
}

// NOTE expects callers to use executorServer Mutex to synchronize for s.handles
func (s *executorServer) addHandle(f io.Closer) int32 {
	s.nextHandlerID++
	s.handles[s.nextHandlerID] = f
	return s.nextHandlerID
}

func (s *executorServer) removeHandle(handlerID int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.handles, handlerID)
}

func (s *executorServer) getWriter(handlerID int32) (io.WriteCloser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.handles[handlerID]
	if !ok {
		return nil, ok
	}
	p, ok := h.(io.WriteCloser)
	return p, ok
}

func (s *executorServer) getReader(handlerID int32) (io.ReadCloser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.handles[handlerID]
	if !ok {
		return nil, ok
	}
	p, ok := h.(io.ReadCloser)
	return p, ok
}

func (s *executorServer) getFile(handlerID int32) (workspace.File, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.handles[handlerID]
	if !ok {
		return nil, ok
	}
	f, ok := h.(workspace.File)
	return f, ok
}

func (s *executorServer) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, h := range s.handles {
		_ = h.Close()
	}
	s.handles = nil
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
