package browser

//go:generate protoc rpc/browser.proto -I=. -I=../ --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/browser.proto=unstable.build/go-tui/browser/rpc
//go:generate mockgen -destination=./test/browser_gomock.go -package test -self_package unstable.build/go-tui/browser/test -source ./browser.go
