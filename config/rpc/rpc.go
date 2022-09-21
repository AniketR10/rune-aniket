package rpc

import (
	"context"
	"fmt"

	"github.com/ernestrc/go-tui/config"
	"google.golang.org/grpc"
)

type configServer struct {
	cfg config.JSON
	UnimplementedConfigServer
}

// NewServer returns a configpb.ConfigServer that serves cfg.
func NewServer(cfg config.Config) ConfigServer {
	ret := new(configServer)
	ret.cfg = config.JSONFromConfig(cfg)
	return ret
}

// Get satisfies configpb.ConfigServer.
func (s *configServer) Get(
	ctx context.Context, req *GetRequest,
) (res *GetResponse, err error) {

	var data []byte
	data, err = s.cfg.MarshalText()
	if err != nil {
		err = fmt.Errorf("Could not marshal config: %w", err)
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
		err = fmt.Errorf("Could not fetch config from server: %w", err)
		return nil, err
	}
	var cfg config.JSON
	err = cfg.UnmarshalText([]byte(res.GetData()))
	if err != nil {
		err = fmt.Errorf("Could unmarshal config from server: %w", err)
		return nil, err
	}
	return cfg, nil
}
