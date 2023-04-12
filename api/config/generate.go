package config

//go:generate protoc rpc/config.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/config.proto=unstable.build/go-tui/api/config/rpc
