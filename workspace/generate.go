package workspace

//go:generate protoc rpc/workspace.proto --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/workspace.proto=github.com/ernestrc/go-tui/workspace/rpc
