package proto

//go:generate protoc term.proto --go_out=. --go-grpc_out=.
//go:generate protoc tui.proto --go_out=. --go-grpc_out=.
//go:generate protoc handler.proto --go_out=. --go-grpc_out=.
//go:generate protoc browser.proto --go_out=. --go-grpc_out=.
//go:generate protoc grantee.proto --go_out=. --go-grpc_out=.
//go:generate protoc editor.proto --go_out=. --go-grpc_out=.
//go:generate protoc editor_event_handler.proto --go_out=. --go-grpc_out=.
//go:generate protoc clipboard.proto --go_out=. --go-grpc_out=.
