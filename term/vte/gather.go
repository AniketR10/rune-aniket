// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package vte

import (
	"context"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"golang.org/x/sys/unix"
	"unstable.build/go-tui/debug"
)

// The gather stage drains the pty on a dedicated thread so the kernel
// queue is never left full while output is parsed or drawn. The macOS
// kernel tty queue holds about 1KiB and hands the master at most that
// much per read, so any pause on the reader immediately blocks the
// child; decoupling reading from parsing is what keeps bulk output
// (cat-ing a large file) flowing at the kernel's rate. The design and
// tuning follow ghostty's two-stage pty pipeline.
const (
	// gatherBatchCount bounds how many batches the gather stage can
	// run ahead of the parse stage before it blocks, which (via the
	// kernel pty queue) preserves flow control to the child.
	gatherBatchCount = 4

	// gatherBatchSize is also the unit of work the parse stage does
	// per batch, so it bounds gather latency and parse chunking.
	gatherBatchSize = 64 * 1024

	// gatherSaturatedRead marks a stream as saturated: the kernel
	// hands the master at most about 1KiB per read, so a full read
	// means the writer filled the queue (a bulk stream worth briefly
	// waiting on), while anything smaller is an interactive trickle
	// that must be delivered with no added latency.
	// Saturation is sticky for the lifetime of a batch: a short read
	// only means the reader caught the writer mid-refill, and demoting
	// the stream back to trickle handling on every such gap degrades
	// bulk output to per-KiB deliveries whose channel wake-ups dwarf
	// the read syscalls themselves.
	gatherSaturatedRead = 1024

	// gatherSpinMax is how many EAGAINs on a saturated stream are
	// retried with an immediate read before sleeping in poll. The
	// writer refills the drained queue within microseconds, while a
	// sleep and wakeup through poll costs several more; sleeping on
	// every refill gap degenerates to lockstep with the writer at
	// about 1KiB per wakeup. Only saturated streams spin, so an idle
	// or interactive terminal never does.
	gatherSpinMax = 16

	// gatherPollTimeoutMs bounds one poll for the writer's next
	// refill once spinning failed; when it expires the batch is
	// delivered with what it has.
	gatherPollTimeoutMs = 1

	// gatherBudget bounds how long one batch may bridge refill gaps
	// before it is delivered regardless, keeping batching well under
	// one display frame.
	gatherBudget = 3 * time.Millisecond
)

type ptyGather struct {
	ctx   context.Context
	fd    int
	quitR int
	quitW int
	free  chan []byte
	ready chan []byte
	// err is set by the gather goroutine before ready is closed and
	// must only be read after ready is drained.
	err error
}

// newPtyGather starts the gather stage for a local pty master. It
// reports false when master does not expose a local tty descriptor (a
// remote workspace or a test fake), in which case the caller must read
// the master directly.
func newPtyGather(ctx context.Context, master workspaceapi.File) (*ptyGather, bool) {
	fd := int(master.Fd())
	if fd <= 2 {
		return nil, false
	}
	if _, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ); err != nil {
		return nil, false
	}
	// A dup keeps raw reads valid while Component.Close closes the
	// master out from under the gather thread; the quit pipe wakes it.
	dup, err := unix.Dup(fd)
	if err != nil {
		return nil, false
	}
	unix.CloseOnExec(dup)
	_ = unix.SetNonblock(dup, true)
	var pipe [2]int
	if err := unix.Pipe(pipe[:]); err != nil {
		_ = unix.Close(dup)
		return nil, false
	}
	unix.CloseOnExec(pipe[0])
	unix.CloseOnExec(pipe[1])
	_ = unix.SetNonblock(pipe[0], true)
	_ = unix.SetNonblock(pipe[1], true)

	g := &ptyGather{
		ctx:   ctx,
		fd:    dup,
		quitR: pipe[0],
		quitW: pipe[1],
		free:  make(chan []byte, gatherBatchCount),
		ready: make(chan []byte, gatherBatchCount),
	}
	for range gatherBatchCount {
		g.free <- make([]byte, gatherBatchSize)
	}
	go debug.CapturePanicReport(func() {
		<-ctx.Done()
		var quit [1]byte
		_, _ = unix.Write(g.quitW, quit[:])
		_ = unix.Close(g.quitW)
	})
	go debug.CapturePanicReport(g.gather)
	return g, true
}

// release returns a consumed batch to the gather stage. The ring has
// exactly gatherBatchCount buffers so the send can never block.
func (g *ptyGather) release(batch []byte) {
	g.free <- batch[:gatherBatchSize]
}

func (g *ptyGather) gather() {
	// The gather loop bypasses the runtime poller with raw reads and
	// short polls; pin it to its own thread like any dedicated IO
	// thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(g.ready)
	defer func() {
		_ = unix.Close(g.fd)
		_ = unix.Close(g.quitR)
	}()

	for {
		var buf []byte
		select {
		case buf = <-g.free:
		case <-g.ctx.Done():
			g.err = g.ctx.Err()
			return
		}
		n, err := g.fill(buf)
		if n > 0 {
			select {
			case g.ready <- buf[:n]:
			case <-g.ctx.Done():
				g.err = g.ctx.Err()
				return
			}
		}
		if err != nil {
			g.err = err
			return
		}
	}
}

// fill gathers one batch. It returns a nil error when the batch should
// be delivered and gathering should continue, and a terminal error
// (io.EOF, EIO, context cancellation) once the stream is over.
func (g *ptyGather) fill(buf []byte) (int, error) {
	var n, spins int
	var start time.Time
	saturated := false
	for n < len(buf) {
		r, err := unix.Read(g.fd, buf[n:])
		if r > 0 {
			if n == 0 {
				start = time.Now()
			}
			n += r
			saturated = saturated || r >= gatherSaturatedRead
			spins = 0
			if time.Since(start) >= gatherBudget {
				return n, nil
			}
			continue
		}
		if r == 0 && err == nil {
			return n, io.EOF
		}
		switch err {
		case unix.EINTR:
			continue
		case unix.EAGAIN:
		default:
			return n, &os.SyscallError{Syscall: "read", Err: err}
		}
		if n > 0 {
			if !saturated {
				return n, nil
			}
			if spins < gatherSpinMax {
				spins++
				continue
			}
		}
		ptyReady, quit, err := g.poll(n > 0)
		if err != nil {
			return n, err
		}
		if quit {
			if err := g.ctx.Err(); err != nil {
				return n, err
			}
			return n, context.Canceled
		}
		if !ptyReady && n > 0 {
			return n, nil
		}
	}
	return n, nil
}

func (g *ptyGather) poll(bounded bool) (ptyReady, quit bool, err error) {
	timeout := -1
	if bounded {
		timeout = gatherPollTimeoutMs
	}
	fds := [2]unix.PollFd{
		{Fd: int32(g.fd), Events: unix.POLLIN},
		{Fd: int32(g.quitR), Events: unix.POLLIN},
	}
	for {
		_, err := unix.Poll(fds[:], timeout)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return false, false, &os.SyscallError{Syscall: "poll", Err: err}
		}
		if fds[1].Revents != 0 {
			return false, true, nil
		}
		return fds[0].Revents != 0, false, nil
	}
}
