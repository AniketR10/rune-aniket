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

package ideauthorizer

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"
	"unstable.build/go-tui/extension/extensionv2/peerprocess"
)

// PeerProcessAddr is a net.Addr that carries information about the process on
// the other end of a Unix-domain socket. The extension runner's listener wraps
// each accepted connection's remote address in a PeerProcessAddr so the
// authorizer can identify the calling plugin by its program path/args/PID.
type PeerProcessAddr struct {
	Addr    net.Addr
	Process peerprocess.Process
	Err     error
}

// Network returns the network of the underlying address, or "unix" if none.
func (a PeerProcessAddr) Network() string {
	if a.Addr != nil {
		return a.Addr.Network()
	}
	return "unix"
}

// String returns the peer process's program path when available, falling back
// to the underlying address.
func (a PeerProcessAddr) String() string {
	if path := a.Process.ProgramPath(); path != "" {
		return path
	}
	if a.Addr != nil {
		return a.Addr.String()
	}
	return ""
}

func peerProcessFromContext(ctx context.Context) (peerprocess.Process, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return peerprocess.Process{}, false
	}
	switch addr := p.Addr.(type) {
	case PeerProcessAddr:
		return usablePeerProcess(addr.Process, addr.Err)
	case *PeerProcessAddr:
		if addr == nil {
			return peerprocess.Process{}, false
		}
		return usablePeerProcess(addr.Process, addr.Err)
	default:
		return peerprocess.Process{}, false
	}
}

func usablePeerProcess(process peerprocess.Process, err error) (peerprocess.Process, bool) {
	if err != nil || process.ProgramPath() == "" {
		return peerprocess.Process{}, false
	}
	return process, true
}
