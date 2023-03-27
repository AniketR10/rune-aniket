package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/ernestrc/blue/logging"
	bluenet "github.com/ernestrc/blue/net"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

var readBufferSize = 1024 * 64

type serverCommandStreamer struct {
	cmd       workspaceapi.Cmd
	stream    Executor_StartCommandServer
	quitCh    chan struct{}
	doneCh    chan error
	stdinCh   chan bluenet.ReadResult
	stdoutCh  chan bluenet.ReadResult
	stderrCh  chan bluenet.ReadResult
	parentCtx context.Context
	closers   []io.Closer
}

func newServerCommandStreamer(
	parentCtx context.Context,
	stream Executor_StartCommandServer,
	path string, args, env []string,
	stdinSet, stdoutSet, stderrSet bool,
) *serverCommandStreamer {
	quitCh := make(chan struct{})
	doneCh := make(chan error)

	cmd := workspaceapi.Cmd{
		Path:    path,
		Args:    args,
		Env:     env,
		Watcher: workspaceapi.ChanWatcher(doneCh),
	}

	ret := new(serverCommandStreamer)

	// it's important that this channels are not buffered
	// so when Cmd.Wait returns, it means that all the data
	// has been drained from the connection
	stdinCh := make(chan bluenet.ReadResult)
	stdoutCh := make(chan bluenet.ReadResult)
	stderrCh := make(chan bluenet.ReadResult)

	// always set these channels so we don't need to worry
	// about nil conditions below
	ret.stdinCh = stdinCh
	ret.stdoutCh = stdoutCh
	ret.stderrCh = stderrCh

	if stdinSet {
		stdinOutCh := make(chan bluenet.ReadResult)
		// use ChanConn as io.Writer and io.Reader,
		// which means that addrs can be nil
		stdin := bluenet.ChanConn(nil, nil /* addrs */, stdinCh, stdinOutCh)
		cmd.Stdin = stdin
		ret.closers = append(ret.closers, stdin)
	}

	if stdoutSet {
		stdoutInCh := make(chan bluenet.ReadResult)
		stdout := bluenet.ChanConn(nil, nil /* addrs */, stdoutInCh, stdoutCh)
		cmd.Stdout = stdout
		ret.closers = append(ret.closers, stdout)
	}

	if stderrSet {
		stderrInCh := make(chan bluenet.ReadResult)
		stderr := bluenet.ChanConn(nil, nil /* addrs */, stderrInCh, stderrCh)
		cmd.Stderr = stderr
		ret.closers = append(ret.closers, stderr)
	}

	ret.cmd = cmd
	ret.stream = stream
	ret.quitCh = quitCh
	ret.doneCh = doneCh

	ret.parentCtx = parentCtx

	return ret
}

func (s *serverCommandStreamer) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithFields(log.Fields{logging.KeyClass: "serverCommandStreamer"}).
		Logf(level, msg, args...)
}

func (s *serverCommandStreamer) receiveCommandData() {
	defer s.log(log.TraceLevel, "done receiving command data")
	defer close(s.stdinCh)
	for {
		var msg CommandPayload
		err := s.stream.RecvMsg(&msg)
		s.log(log.TraceLevel, "receive msg: err=%v", err)
		if err != nil {
			if err == io.EOF {
				return
			}
			if cerr := s.stream.Context().Err(); cerr != nil {
				return
			}
			select {
			case <-s.parentCtx.Done():
				return
			case <-s.quitCh:
				return
			default:
				ack := make(chan struct{})
				select {
				case s.stdinCh <- bluenet.ReadResult{Error: err, Ch: ack}:
					<-ack
					continue
				case <-s.parentCtx.Done():
					return
				case <-s.quitCh:
					return
				}
			}
		}

		var data []byte
		switch msg.Type {
		case CommandPayload_TypeIO:
			io := msg.GetIo()
			switch io.GetType() {
			case CommandPayload_IO_TypeStdin:
				data = io.GetData()
			default:
				err = fmt.Errorf("unexpected IO type received: %v", io.GetType())
			}
		default:
			err = fmt.Errorf("unexpected message type received: %v", msg.Type)
		}

		ack := make(chan struct{})
		select {
		case s.stdinCh <- bluenet.ReadResult{Error: err, Data: data, Ch: ack}:
			<-ack
			s.log(log.TraceLevel,
				"wrote to stdin: err=%v, data=%d", err, len(data))
			continue
		case <-s.parentCtx.Done():
			return
		case <-s.quitCh:
			return
		}
	}
}

func (s *serverCommandStreamer) streamReadResult(
	res bluenet.ReadResult, t CommandPayload_IO_Type,
) error {
	defer close(res.Ch)

	if res.Error != nil {
		err := s.stream.Send(&CommandPayload{
			Type:  CommandPayload_TypeError,
			Error: res.Error.Error(),
		})
		s.log(log.TraceLevel, "send error msg: err=%v", err)
		if err != nil {
			return fmt.Errorf("send error msg: %v", err)
		}
		return fmt.Errorf("error reading from standard io %v: %v", t, res.Error)
	}
	err := s.stream.Send(&CommandPayload{
		Type: CommandPayload_TypeIO,
		Io:   &CommandPayload_IO{Data: res.Data, Type: t},
	})
	s.log(log.TraceLevel, "send io msg: err=%v", err)
	if err != nil {
		return fmt.Errorf("send io msg: %v", err)
	}
	return nil
}

func (s *serverCommandStreamer) sendCommandData(pid workspaceapi.Pid) error {
	err := s.stream.Send(&CommandPayload{
		Type: CommandPayload_TypeStarted,
		Started: &CommandPayload_Started{
			Pid: int64(pid),
		},
	})
	s.log(log.TraceLevel, "send started msg: err=%v", err)
	if err != nil {
		return fmt.Errorf("send msg started: %v", err)
	}
	for {
		var err error
		select {
		case <-s.parentCtx.Done():
			err = errors.New("workspace is closing")
		case <-s.quitCh:
			err = errors.New("called Close but command is not done")
		case res, ok := <-s.stdoutCh:
			s.log(log.TraceLevel, "read from stdout: ok=%v, err=%v, data=%d",
				ok, res.Error, len(res.Data))
			if !ok {
				return nil
			}
			err = s.streamReadResult(res, CommandPayload_IO_TypeStdout)
		case res, ok := <-s.stderrCh:
			s.log(log.TraceLevel, "read from stderr: ok=%v, err=%v, data=%d",
				ok, res.Error, len(res.Data))
			if !ok {
				return nil
			}
			err = s.streamReadResult(res, CommandPayload_IO_TypeStderr)
		case doneErr := <-s.doneCh:
			var errStr string
			if doneErr != nil && doneErr != io.EOF {
				errStr = doneErr.Error()
			}
			err = s.stream.Send(&CommandPayload{
				Type: CommandPayload_TypeDone,
				Done: &CommandPayload_Done{
					ExitError: errStr,
				},
			})
			if err != nil {
				err = fmt.Errorf("send done msg: %v", err)
				s.log(log.WarnLevel, "%v", err)
			} else {
				s.log(log.TraceLevel, "send done msg: success")
			}
			return err
		}
		if err != nil {
			return err
		}
	}
}

func (s *serverCommandStreamer) command() workspaceapi.Cmd {
	return s.cmd
}

func (s *serverCommandStreamer) Close() (ret error) {
	for _, closer := range s.closers {
		if err := closer.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	close(s.quitCh)
	return
}

type clientCommandStreamer struct {
	cmd       workspaceapi.Cmd
	stream    Executor_StartCommandClient
	parentCtx context.Context
	quitCh    chan struct{}
}

func newClientCommandStreamer(
	parentCtx context.Context,
	cmd workspaceapi.Cmd,
	stream Executor_StartCommandClient,
) *clientCommandStreamer {
	ret := new(clientCommandStreamer)
	ret.cmd = cmd
	ret.stream = stream
	ret.parentCtx = parentCtx
	ret.quitCh = make(chan struct{})
	return ret
}

func (s *clientCommandStreamer) waitForPid() (workspaceapi.Pid, error) {
	var msg CommandPayload
	err := s.stream.RecvMsg(&msg)
	s.log(log.TraceLevel, "receive first msg: err=%v", err)
	if err != nil {
		return 0, fmt.Errorf("stream receive msg: %v", err)
	}

	if msg.Type != CommandPayload_TypeStarted || msg.Started == nil {
		return 0, fmt.Errorf("expected stream started msg, found %v", msg.Type)
	}

	return workspaceapi.Pid(msg.Started.Pid), nil
}

func (s *clientCommandStreamer) streamStdin() {
	defer s.stream.CloseSend()

	if s.cmd.Stdin == nil {
		s.log(log.TraceLevel, "no stdin set in cmd, skipping streaming stdin")
		return
	}

	var err error
	buf := make([]byte, readBufferSize)
	for {
		select {
		case <-s.quitCh:
			return
		case <-s.parentCtx.Done():
			s.log(log.TraceLevel, "parent context is done")
			return
		default:
		}

		var n int
		n, err = s.cmd.Stdin.Read(buf)
		s.log(log.TraceLevel, "read from stdin: err=%v, data=%d", err, n)
		if err != nil && err != io.EOF {
			break
		}
		serr := s.stream.Send(&CommandPayload{
			Type: CommandPayload_TypeIO,
			Io: &CommandPayload_IO{
				Data: buf[:n],
				Type: CommandPayload_IO_TypeStdin,
			},
		})
		if serr != nil {
			err = fmt.Errorf("stream send: %v", serr)
			break
		}
		if err == io.EOF {
			err = nil
			break
		}
	}

	if err != nil {
		s.log(log.ErrorLevel, "stream stdin stopped with error: %v", err)
	}
}

func (s *clientCommandStreamer) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{logging.KeyClass: "clientCommandStreamer"}).Logf(level, msg, args...)
}

func (s *clientCommandStreamer) streamCommandData(client interface{}) {
	s.log(log.TraceLevel, "streaming command data")
	defer close(s.quitCh)

	go s.streamStdin()

	var err error
	for {
		select {
		case <-s.parentCtx.Done():
			s.log(log.TraceLevel,
				"parent context is done, canceling streaming cmd data")
			return
		default:
		}

		var msg CommandPayload
		err = s.stream.RecvMsg(&msg)
		s.log(log.TraceLevel, "receive msg: err=%v", err)
		if err != nil {
			if err == io.EOF {
				break
			}
			err = fmt.Errorf("stream receive msg: %v", err)
			break
		}

		var n int
		switch msg.Type {
		case CommandPayload_TypeIO:
			switch msg.GetIo().GetType() {
			case CommandPayload_IO_TypeStdout:
				if s.cmd.Stdout == nil {
					s.log(log.TraceLevel, "no stdout set in cmd, dropping data")
					continue
				}
				if msg.GetIo().Data == nil {
					s.log(log.WarnLevel, "stdout type without stdout data")
					continue
				}
				n, err = s.cmd.Stdout.Write(msg.GetIo().GetData())
				s.log(log.TraceLevel, "wrote to stdout, err=%v, data=%d", err, len(msg.GetIo().GetData()))
				if err != nil {
					err = fmt.Errorf("stdout io.Writer write: %v", err)
				}
			case CommandPayload_IO_TypeStderr:
				if s.cmd.Stderr == nil {
					s.log(log.TraceLevel, "no stderr set in cmd, dropping data")
					continue
				}
				if msg.GetIo().Data == nil {
					s.log(log.WarnLevel, "stderr type without stderr data")
					continue
				}
				n, err = s.cmd.Stderr.Write(msg.GetIo().GetData())
				s.log(log.TraceLevel, "wrote to stderr, err=%v, data=%d", err, n)
				if err != nil {
					err = fmt.Errorf("stderr io.Writer write: %v", err)
				}
			default:
				err = fmt.Errorf("unexpected io message received %v", msg.GetType())
			}
			if err == nil {
				continue
			}
		case CommandPayload_TypeError:
			err = fmt.Errorf("error reading command stdio: %v", msg.GetError())
		case CommandPayload_TypeDone:
			if msg.GetDone().GetExitError() != "" {
				err = errors.New(msg.GetDone().GetExitError())
			}
		default:
			err = fmt.Errorf("unexpected message received %v", msg.GetType())
		}
		break
	}

	if s.cmd.Watcher != nil {
		select {
		case s.cmd.Watcher.Watch() <- err:
		case <-s.parentCtx.Done():
		}
	}
	// emulate exec code; pipes should be closed to force EOF
	for _, fd := range [2]interface{}{s.cmd.Stdout, s.cmd.Stderr} {
		if closer, ok := fd.(*os.File); ok {
			_ = closer.Close()
		}
	}
	s.log(log.TraceLevel, "done streaming command data: err=%v", err)
	runtime.KeepAlive(client)
}
