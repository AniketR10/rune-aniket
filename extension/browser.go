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

package extension

import (
	"io"
	"sync"

	"unstable.build/go-tui/browser"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

// BrowserResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Browser's resources.
func BrowserResources(
	b browser.Browser, publishEvent func(term.Event) bool,
) map[Permission]ResourceRegistrar {
	s := newBrowserResourceServer(b, publishEvent)
	return map[Permission]ResourceRegistrar{
		PermissionBrowserWindowManager: s.forPermission(
			PermissionBrowserWindowManager),
		PermissionBrowserResourceOpener: s.forPermission(
			PermissionBrowserResourceOpener),
		PermissionBrowserNotifications: s.forPermission(
			PermissionBrowserNotifications),
		PermissionBrowserEventPublisher: s.forPermission(
			PermissionBrowserEventPublisher),
	}
}

type browserResourceServer struct {
	b            browser.Browser
	publishEvent func(term.Event) bool
}

type browserResourcePermissionServer struct {
	p Permission
	*browserResourceServer
}

func newBrowserResourceServer(
	b browser.Browser, publishEvent func(term.Event) bool,
) *browserResourceServer {
	ret := new(browserResourceServer)
	ret.b = b
	ret.publishEvent = publishEvent
	return ret
}

func (s *browserResourceServer) forPermission(p Permission) ResourceRegistrar {
	return browserResourcePermissionServer{p: p, browserResourceServer: s}
}

func (s browserResourcePermissionServer) Register(
	extensionID string, grantor Grantor, registrar rpc.ServiceRegistrar,
	broker rpc.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	server := browserpb.NewServer(broker, s.b, lock)
	rpcServer := interruptBrowserServer(server, func() {
		s.publishEvent(term.Event{Type: term.EventInterrupt})
	})
	switch s.p {
	case PermissionBrowserWindowManager:
		browserpb.RegisterWindowManagerServer(registrar, rpcServer)
	case PermissionBrowserResourceOpener:
		browserpb.RegisterResourceOpenerServer(registrar, rpcServer)
	case PermissionBrowserNotifications:
		browserpb.RegisterNotificationsServer(registrar, rpcServer)
	case PermissionBrowserEventPublisher:
		browserpb.RegisterEventPublisherServer(registrar, rpcServer)
	}
	return browserCloser{server}, nil
}

type browserCloser struct {
	server *browserpb.Server
}

func (b browserCloser) Close() error {
	return b.server.Stop()
}
