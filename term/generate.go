package term

//go:generate protoc rpc/term.proto --go_out=./ --go-grpc_out=./ --go_opt=Mrpc/term.proto=github.com/ernestrc/go-tui/term/rpc
