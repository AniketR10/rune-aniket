package proto

//go:generate protoc term.proto --go_out=plugins=grpc:.
//go:generate protoc tui.proto --go_out=plugins=grpc:.
//go:generate protoc handler.proto --go_out=plugins=grpc:.
//go:generate protoc browser.proto --go_out=plugins=grpc:.
//go:generate protoc grantee.proto --go_out=plugins=grpc:.
