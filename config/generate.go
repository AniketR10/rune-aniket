package config

//go:generate protoc rpc/config.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/config.proto=github.com/ernestrc/go-tui/config/rpc
