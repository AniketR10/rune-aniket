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
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	sdkllmrpc "github.com/unstablebuild/rune-go-sdk/api/llmapi/llmrpc"
	"unstable.build/go-tui/llm/llmrpc"
	"unstable.build/go-tui/rpc"
)

// LLMResources returns a map of Permission to a ResourceRegistrar capable
// of serving the given llmapi.Service to extensions and plugins.
func LLMResources(svc llmapi.Service) map[extensionapi.Permission]ResourceRegistrar {
	s := &llmResourceServer{svc: svc}
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionLLM: s,
	}
}

type llmResourceServer struct {
	svc llmapi.Service
}

func (s *llmResourceServer) Register(
	registrar rpc.ServiceRegistrar, _ sync.Locker,
) (io.Closer, error) {
	server := llmrpc.NewServer(s.svc)
	sdkllmrpc.RegisterLLMServer(registrar, server)
	return server, nil
}
