package browser

//go:generate protoc rpc/browser.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/browser.proto=github.com/ernestrc/go-tui/browser/rpc
