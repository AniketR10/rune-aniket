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

	tsemanticrpc "github.com/unstablebuild/idelsp/semanticrpc"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi/semanticrpc"
	"unstable.build/go-tui/rpc"
)

// SemanticResources returns a map of Permission to a ResourceServer
// capable of serving each of the b SemanticTree's resources.
func SemanticResources(b semanticapi.LSP) map[extensionapi.Permission]ResourceRegistrar {
	s := newSemanticTreeResourceServer(b)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionLSP: s.forPermission(
			extensionapi.PermissionLSP),
	}
}

type semanticResourceServer struct {
	b semanticapi.LSP
}

type semanticResourcePermissionServer struct {
	p extensionapi.Permission
	*semanticResourceServer
}

func newSemanticTreeResourceServer(b semanticapi.LSP) *semanticResourceServer {
	ret := new(semanticResourceServer)
	ret.b = b
	return ret
}

func (s *semanticResourceServer) forPermission(p extensionapi.Permission) ResourceRegistrar {
	return semanticResourcePermissionServer{p: p, semanticResourceServer: s}
}

func (s semanticResourcePermissionServer) Register(
	registrar rpc.ServiceRegistrar, lock sync.Locker,
) (io.Closer, error) {
	server := tsemanticrpc.NewServer(s.b)
	switch s.p {
	case extensionapi.PermissionLSP:
		semanticrpc.RegisterLSPServer(registrar, server)
	default:
		return nil, errors.New("unknown permission for semantic server")
	}
	return server, nil
}
