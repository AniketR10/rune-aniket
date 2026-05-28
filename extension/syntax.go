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

package extension

import (
	"errors"
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	tsyntaxrpc "unstable.build/go-tui/ide/syntax/syntaxrpc"
	"unstable.build/go-tui/rpc"
)

// SyntaxResources returns a map of Permission to a ResourceServer
// capable of serving each of the b SyntaxTree's resources.
func SyntaxResources(b syntaxapi.Parser) map[extensionapi.Permission]ResourceRegistrar {
	s := newSyntaxTreeResourceServer(b)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionSyntaxTree: s.forPermission(
			extensionapi.PermissionSyntaxTree),
	}
}

type syntaxResourceServer struct {
	b syntaxapi.Parser
}

type syntaxResourcePermissionServer struct {
	p extensionapi.Permission
	*syntaxResourceServer
}

func newSyntaxTreeResourceServer(b syntaxapi.Parser) *syntaxResourceServer {
	ret := new(syntaxResourceServer)
	ret.b = b
	return ret
}

func (s *syntaxResourceServer) forPermission(p extensionapi.Permission) ResourceRegistrar {
	return syntaxResourcePermissionServer{p: p, syntaxResourceServer: s}
}

func (s syntaxResourcePermissionServer) Register(
	registrar rpc.ServiceRegistrar, locker sync.Locker,
) (io.Closer, error) {
	server := tsyntaxrpc.NewServer(s.b, locker)
	switch s.p {
	case extensionapi.PermissionSyntaxTree:
		syntaxrpc.RegisterSyntaxServer(registrar, server)
	default:
		return nil, errors.New("unknown permission for syntax server")
	}
	return server, nil
}
