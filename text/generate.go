package text

//go:generate protoc rpc/editor.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/editor.proto=unstable.build/go-tui/text/rpc
//go:generate mockgen -destination=./test/event_handler_gomock.go -package test -self_package unstable.build/go-tui/text/test -source ./event_handler.go
//go:generate mockgen -destination=./test/editor_gomock.go -package test -self_package unstable.build/go-tui/text/test -source ./editor.go
//go:generate mockgen -destination=./test/event_handler_gomock.go -package test -self_package unstable.build/go-tui/text/test -source ./event_handler.go
//go:generate mockgen -destination=./test/mouse_gomock.go -package test -self_package unstable.build/go-tui/text/test -source ./mouse.go
