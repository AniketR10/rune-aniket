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
	"fmt"
	"io"
	"sync"

	"unstable.build/go-tui/rpc"
)

// ResourceRegistrar wraps the basic Serve method, to serve resources over a mux broker.
type ResourceRegistrar interface {
	Register(
		extensionID string, grantor Grantor,
		registar rpc.ServiceRegistrar, broker rpc.MuxBroker,
		locker sync.Locker) (io.Closer, error)
}

// MergeResourceMap merges m1 with mn.
// If permissions are overlapping, the last of passed prevails.
func MergeResourceMap(
	m1 map[Permission]ResourceRegistrar, mn ...map[Permission]ResourceRegistrar,
) map[Permission]ResourceRegistrar {
	ret := make(map[Permission]ResourceRegistrar)
	for k, v := range m1 {
		ret[k] = v
	}
	for _, m := range mn {
		for k, v := range m {
			ret[k] = v
		}
	}
	return ret
}

// Grantor encapsulates the ability grant or deny access to resources.
type Grantor interface {
	Grant(extension string, perm Permission) (ResourceRegistrar, bool)
}

type inmemoryGrantor struct {
	mu     sync.Mutex
	grants map[string]Permissions
	res    map[Permission]ResourceRegistrar
}

// NewInmemoryGrantor returns a Grantor that Grants according to the given
// grants and resources maps. This function panics if there's a Permission in
// grants that does not have a ResourceServe in res.
func NewInmemoryGrantor(
	grants map[string]Permissions,
	res map[Permission]ResourceRegistrar,
) Grantor {
	ret := new(inmemoryGrantor)
	ret.init(grants, res)
	return ret
}

func (m *inmemoryGrantor) init(
	grants map[string]Permissions,
	res map[Permission]ResourceRegistrar,
) {
	m.grants = grants
	m.res = res

	for _, extensionGrants := range m.grants {
		for grant := range extensionGrants {
			if _, ok := m.res[grant]; !ok {
				panic(fmt.Sprintf("grantor: resource registrar not found for grant: %s", grant))
			}
		}
	}
}

func (m *inmemoryGrantor) Grant(extension string, perm Permission) (ResourceRegistrar, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	grant, ok := m.grants[extension]
	if !ok {
		return nil, false
	}

	_, ok = grant[perm]
	if !ok {
		return nil, false
	}

	return m.res[perm], true
}

type grantAll struct {
	caps map[Permission]ResourceRegistrar
}

func (g *grantAll) Grant(extension string, perm Permission) (ResourceRegistrar, bool) {
	srv, ok := g.caps[perm]
	return srv, ok
}

// GrantAll returns Grantor that Grants permission to all the given capabilities.
func GrantAll(capabilities map[Permission]ResourceRegistrar) Grantor {
	return &grantAll{caps: capabilities}
}
