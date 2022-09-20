package text

//go:generate protoc rpc/editor.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/editor.proto=github.com/ernestrc/go-tui/text/rpc
