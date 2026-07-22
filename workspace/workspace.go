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

//revive:disable:exported
package workspace

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cell"
)

// Workspace binds a Loader and a Scheme together for use in internal
// packages that both need to share an API with external resources
// and use a Loader to load resources into buffers.
type Workspace interface {
	Loader
	schemeapi.Scheme
}

// RemoteScheme is implemented by schemes whose underlying transport
// can drop and reconnect (e.g. SSH). The signal published by
// OnDisconnect must be interpreted as "every file descriptor handed out
// by this scheme up to now is invalid": clients are expected to
// drop cached handles, reset pools and reload whatever they need
// against a freshly-resolved transport on demand.
//
// Each call to OnDisconnect returns a channel that is closed once on
// the next transport drop. Subsequent calls return a fresh channel
// for the next transition, so a single caller can re-arm after
// reacting to a drop:
//
//	for {
//	    select {
//	    case <-rs.OnDisconnect():
//	        invalidate()
//	    case <-ctx.Done():
//	        return
//	    }
//	}
//
// Local schemes do not implement RemoteScheme; callers should type-
// assert and degrade silently when the assertion fails.
type RemoteScheme interface {
	OnDisconnect() <-chan struct{}

	// WaitConnected blocks until the transport state is resolved:
	// the first connection attempt has settled (successfully or
	// not), the scheme is closed, or ctx is done. It does NOT
	// guarantee the transport is healthy — only that scheme calls
	// issued afterwards will not block on connection establishment
	// (which is unbounded: first-connect provisioning may install
	// packages on the remote host). Callers that apply their own
	// deadline to scheme RPCs should wait here first so the
	// deadline measures the RPC, not the connect.
	WaitConnected(ctx context.Context) error
}

// Loader abstracts the ability to load resource data into a working buffer
// and provide a FlusherCloser to manage flushing data to storage.
type Loader interface {
	Load(file workspaceapi.URI, buf *cell.Buffer, swapDir workspaceapi.URI, readOnly bool) (FlusherCloser, error)
	Recover(file, swapFilePath workspaceapi.URI, buf *cell.Buffer, force bool) (FlusherCloser, error)
	Remove(file string) error
}

// SchemeManager abstracts the ability to register new URI schemes.
type SchemeManager interface {
	RegisterScheme(string, schemeapi.SchemeFunc) error
	UnregisterScheme(string) error
}

// WorkspaceManager abstracts the ability to register schemes and workspaces.
type WorkspaceManager interface {
	SchemeManager
	AddWorkspace(context.Context, workspaceapi.URI) (Workspace, error)
	Workspace(workspaceapi.URI) (Workspace, bool, error)
	// IncrementReference bumps the reference count of the workspace
	// registered under uri. A workspace whose reference count has
	// ever been incremented is owned by its callers and will be
	// closed when DecrementReference brings the count back to zero.
	// Workspaces that are never incremented (e.g. the IDE-owned
	// active workspace) are not subject to refcount teardown.
	IncrementReference(workspaceapi.URI)
	// DecrementReference decrements the reference count of the
	// workspace registered under uri and closes the workspace when
	// the count reaches zero. Decrementing a uri that was never
	// incremented is a no-op.
	DecrementReference(workspaceapi.URI) error
	// RemoveWorkspace unregisters uri and returns the underlying
	// Workspace so the caller can close it off the event loop. The
	// manager's maps are only ever touched on the event loop.
	RemoveWorkspace(workspaceapi.URI) (Workspace, bool)
}

var ErrOpenInOtherWorkspace = errors.New("file should be opened in another workspace")

// ErrFlushInProgress is returned by Flush, ForceFlush and Reload when a
// previous async flush or reload for the same buffer has not yet
// completed.
var ErrFlushInProgress = errors.New(
	"save or reload already in progress for this buffer")

// ErrNoFlushInProgress is returned by callers that need to cancel an
// in-flight flush/reload when there is nothing pending. It is not
// produced by FlusherCloser itself; it is exported here so that
// higher layers can share a single sentinel.
var ErrNoFlushInProgress = errors.New("no save in progress for this buffer")

// FlusherCloser wraps methods to manipulate a cell.Buffer's persistence.
//
// Flush, ForceFlush and Reload are asynchronous. Each returns a buffered
// channel (capacity 1) that will receive exactly one value and then
// close:
//   - nil on success,
//   - workspaceapi.ErrStaleData / ErrFileIsNotWritable / scheme errors,
//   - ctx.Err() if ctx is cancelled before the operation completes.
//
// If a flush (or reload) for this buffer is already in flight, the
// method returns a nil channel and ErrFlushInProgress; the caller must
// wait for the previous operation to complete (or cancel its ctx)
// before retrying.
//
// Cancelling ctx signals disinterest in the result. The underlying
// scheme calls (for example, gRPC Rename) cannot themselves be aborted
// today: the work goroutine continues until the transport responds,
// at which point its result is discarded and a new flush may be
// started.
//
// Reload's result channel only fires after the post-read cell.Buffer
// mutations have been dispatched onto the host event loop and
// completed. This guarantees that buffer subscribers (which may touch
// UI-owned state from OnWillEdit/OnDidEdit) run on the event-loop
// goroutine rather than on the async worker that performed disk I/O.
type FlusherCloser interface {
	Flush(ctx context.Context) (<-chan error, error)
	ForceFlush(ctx context.Context) (<-chan error, error)
	Reload(ctx context.Context) (<-chan error, error)
	LastFlush() time.Time
	io.Closer
}
