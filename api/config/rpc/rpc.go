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

package rpc

import (
	"context"
	"fmt"
	sync "sync"

	"google.golang.org/grpc"
	"unstable.build/go-tui/api/config"
)

type configServer struct {
	UnimplementedConfigServer
	cfg    config.JSON
	locker sync.Locker
}

// NewServer returns a configpb.ConfigServer that serves cfg.
func NewServer(cfg config.Config, locker sync.Locker) ConfigServer {
	ret := new(configServer)
	ret.cfg = config.JSONFromConfig(cfg)
	ret.locker = locker
	return ret
}

// Get satisfies configpb.ConfigServer.
func (s *configServer) Get(
	ctx context.Context, req *GetRequest,
) (res *GetResponse, err error) {
	s.locker.Lock()
	defer s.locker.Unlock()

	var data []byte
	data, err = s.cfg.MarshalText()
	if err != nil {
		err = fmt.Errorf("marshal config: %w", err)
		return
	}

	res = new(GetResponse)
	res.Data = string(data)
	return
}

// FetchConfig fetches a config.Config from a configpb.ConfigServer over
// the given connection.
func FetchConfig(cc grpc.ClientConnInterface) (config.Config, error) {
	client := NewConfigClient(cc)
	req := GetRequest{}
	res, err := client.Get(context.Background(), &req)
	if err != nil {
		err = fmt.Errorf("fetch config from server: %w", err)
		return nil, err
	}
	var cfg config.JSON
	err = cfg.UnmarshalText([]byte(res.GetData()))
	if err != nil {
		err = fmt.Errorf("unmarshal config from server: %w", err)
		return nil, err
	}
	return cfg, nil
}
