package handler

//go:generate protoc rpc/handler.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/handler.proto=unstable.build/go-tui/handler/rpc
