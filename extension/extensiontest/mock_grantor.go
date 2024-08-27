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

package extensiontest

import (
	"io"
	"sync"

	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
)

var _ extension.ResourceRegistrar = (*MockResourceServer)(nil)

// MockResourceServer satisfies extension.ResourceRegistrar for testing.
type MockResourceServer struct {
	mu    sync.Mutex
	muxes []rpc.MuxBroker
}

func (s *MockResourceServer) Register(
	extensionID string, g extension.Grantor, grantor rpc.ServiceRegistrar,
	mux rpc.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.muxes = append(s.muxes, mux)
	return nopCloser{}, nil
}

func (s *MockResourceServer) Muxes() []rpc.MuxBroker {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.muxes
}

func (s *MockResourceServer) Close() error {
	return nil
}

// MockGrantor satisfies extension.Grantor for testing.
type MockGrantor struct {
	mu   sync.Mutex
	srvs []*MockResourceServer
}

func (g *MockGrantor) Servers() []*MockResourceServer {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.srvs
}

func (g *MockGrantor) Grant(extension string, perm extension.Permission) (extension.ResourceRegistrar, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	server := &MockResourceServer{}
	g.srvs = append(g.srvs, server)

	return server, true
}

type nopCloser struct {
}

func (c nopCloser) Close() error {
	return nil
}
