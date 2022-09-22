package workspace

//go:generate protoc rpc/workspace.proto --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/workspace.proto=github.com/ernestrc/go-tui/workspace/rpc
//go:generate mockgen -destination=./test/workspace_gomock.go -package test -self_package github.com/ernestrc/go-tui/workspace/test -source ./workspace.go
