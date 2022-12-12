package tui

// TODO move gomocks to <package>/test sub-package
//go:generate mockgen -destination=./proto/grpc_gomock.go -package proto google.golang.org/grpc ClientConnInterface
//go:generate mockgen -destination=./proto/gomock_broker.go -package proto -self_package unstable.build/go-tui/proto -source ./proto/broker.go
//go:generate mockgen -destination=./plugin/closer_gomock_test.go -package plugin -self_package unstable.build/go-tui/plugin -source ./plugin/clipboard_rpc_test.go
//go:generate mockgen -destination=./plugin/clipboard_gomock_test.go -package plugin -self_package unstable.build/go-tui/plugin -source ./plugin/clipboard.go
