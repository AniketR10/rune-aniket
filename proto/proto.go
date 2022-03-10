package proto

//go:generate protoc term.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc tui.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc handler.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc browser.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc grantee.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc editor.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc editor_event_handler.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
//go:generate protoc clipboard.proto --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative
