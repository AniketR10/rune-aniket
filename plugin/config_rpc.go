package plugin

import (
	"context"
	"fmt"

	configpb "github.com/ernestrc/go-tui/plugin/proto"
	"google.golang.org/grpc"
)

type configServer struct {
	cfg jsonMap
	configpb.UnimplementedConfigServer
}

func newConfigServer(cfg Config) *configServer {
	ret := new(configServer)
	m := toInternalConfig(cfg)
	ret.cfg.mapConfig = m
	return ret
}

func (s *configServer) Get(
	ctx context.Context, req *configpb.GetRequest,
) (res *configpb.GetResponse, err error) {

	var data []byte
	data, err = s.cfg.MarshalText()
	if err != nil {
		err = fmt.Errorf("Could not marshal config: %w", err)
		return
	}

	res = new(configpb.GetResponse)
	res.Data = string(data)
	return
}

func newConfigFromServer(cc grpc.ClientConnInterface) (Config, error) {
	client := configpb.NewConfigClient(cc)
	req := configpb.GetRequest{}
	res, err := client.Get(context.Background(), &req)
	if err != nil {
		err = fmt.Errorf("Could not fetch config from server: %w", err)
		return nil, err
	}
	var cfg jsonMap
	err = cfg.UnmarshalText([]byte(res.GetData()))
	if err != nil {
		err = fmt.Errorf("Could unmarshal config from server: %w", err)
		return nil, err
	}
	return cfg.mapConfig, nil
}
