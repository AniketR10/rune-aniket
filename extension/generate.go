package extension

//go:generate protoc rpc/grantee.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/grantee.proto=unstable.build/go-tui/extension/rpc
