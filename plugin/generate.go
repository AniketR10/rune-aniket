package plugin

//go:generate protoc rpc/grantee.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/grantee.proto=github.com/ernestrc/go-tui/plugin/rpc
//go:generate protoc rpc/clipboard.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/clipboard.proto=github.com/ernestrc/go-tui/plugin/rpc
