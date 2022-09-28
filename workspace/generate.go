package workspace

//go:generate protoc rpc/workspace.proto --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/workspace.proto=unstable.build/go-tui/workspace/rpc
//go:generate mockgen -destination=./test/workspace_gomock.go -package test -self_package unstable.build/go-tui/workspace/test -source ./workspace.go
