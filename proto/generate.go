package proto

//go:generate protoc ./term.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/term.proto=/proto
//go:generate protoc ./tui.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/tui.proto=/proto
//go:generate protoc ./handler.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/handler.proto=/proto
//go:generate protoc ./browser.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/browser.proto=/proto
//go:generate protoc ./editor.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/editor.proto=/proto
//go:generate protoc ./editor_event_handler.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/editor_event_handler.proto=/proto
//go:generate protoc ./clipboard.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/clipboard.proto=/proto
//go:generate protoc ./grantee.proto --go_out=. --go-grpc_out=. --go_opt=Mproto/grantee.proto=/proto
