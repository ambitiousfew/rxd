package rxd

import (
	"net/http"
	"net/rpc"
	"strconv"

	"github.com/ambitiousfew/rxd/log"
)

type RPCConfig struct {
	Addr string
	Port uint16
}

type RPCServer struct {
	server *http.Server
}

func (s *RPCServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *RPCServer) Stop() error {
	return s.server.Close()
}

func NewRPCHandler(cfg RPCConfig) (*RPCServer, error) {
	mux := http.NewServeMux()

	rpcServer := rpc.NewServer()

	rxCommandService := CommandHandler{}

	err := rpcServer.Register(rxCommandService)
	if err != nil {
		return nil, err
	}

	addr := cfg.Addr + ":" + strconv.Itoa(int(cfg.Port))

	return &RPCServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}, nil
}

type CommandHandler struct {
	logger log.Logger // daemon logger
}

func (h CommandHandler) ChangeLogLevel(level log.Level, resp *error) error {
	h.logger.SetLevel(level)
	return nil
}

// func (h CommandHandler) Send(payload rxrpc.CommandPayload, reply *rxrpc.CommandResponse) error {
// 	// retrieve the service's state channel it uses to listen for rxd-specific state transitions.
// 	// current := s.sw.Current()

// 	if payload.Service == "" {
// 		payload.Service = "_all"
// 	}

// 	state, ok := current[payload.Service]

// 	if !ok && payload.Service != "_all" {
// 		reply.Success = false
// 		return errors.New("service not found: " + payload.Service)
// 	}

// 	resp := rpc.RxCommandResponse{
// 		Command: payload.Command,
// 	}

// 	// TODO: to fully support start, stop, restart such as fully exiting the service
// 	// and starting it in a new routine from scratch, we need to add a way to
// 	// run the service broker from here?
// 	switch payload.Command {
// 	case rxrpc.Start:
// 		// stateC <- Init // send the start command to the service's state channel.
// 		s.signalC <- daemonSignal{service: payload.Service, signal: SignalStart}
// 		return errors.New("not implemented")

// 	case rxrpc.Stop:
// 		s.signalC <- daemonSignal{service: payload.Service, signal: SignalStop}

// 	case rxrpc.Restart:
// 		s.signalC <- daemonSignal{service: payload.Service, signal: SignalRestart}

// 	case rxrpc.Reload:
// 		s.signalC <- daemonSignal{service: payload.Service, signal: SignalReload}

// 	case rxrpc.List:
// 		services := make([]string, 0, len(current))
// 		for name := range current {
// 			services = append(services, name)
// 		}
// 		resp.Body = []byte(strings.Join(services, ", "))

// 	case rxrpc.Status:
// 		if payload.Service == "_all" {
// 			body, err := json.Marshal(current)
// 			if err != nil {
// 				return err
// 			}
// 			resp.Body = body
// 		} else {
// 			resp.Body = []byte(state.String())
// 		}

// 	default:
// 		return errors.New("unknown command")
// 	}

// 	resp.Success = true
// 	*reply = resp
// 	return nil
// }
