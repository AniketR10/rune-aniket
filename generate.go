package tui

// TODO move gomocks to <package>/test sub-package
//go:generate mockgen -destination=./rpc/grpc_gomock.go -package rpc google.golang.org/grpc ClientConnInterface
//go:generate mockgen -destination=./rpc/gomock_broker.go -package rpc -self_package unstable.build/go-tui/rpc -source ./rpc/broker.go
