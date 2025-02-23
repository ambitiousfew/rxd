package rxd

import "github.com/ambitiousfew/rxd/pkg/rpc"

type RPCCommandRequest struct {
	Service string
	Signal  rpc.CommandSignal
}

type RPCCommandResponse struct {
	Success bool
	Message []byte
}
