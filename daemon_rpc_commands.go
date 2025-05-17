package rxd

import "github.com/ambitiousfew/rxd/v2/pkg/rpc"

type RPCCommandRequest struct {
	Service string
	Signal  rpc.CommandSignal
}

type RPCCommandResponse struct {
	Success bool
	Message []byte
}
