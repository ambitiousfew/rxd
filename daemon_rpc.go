package rxd

import (
	"net/http"
	"net/rpc"
	"strconv"

	"github.com/ambitiousfew/rxd/log"
)

// RPCConfig holds the configuration for the RPC server.
// It includes the address and port on which the server will listen.
// The address can be an IP or a hostname, and the port is a uint16.
// The server will listen on the specified address and port for incoming RPC requests.
type RPCConfig struct {
	Addr string
	Port uint16
}

// RPCServer is a struct that represents an RPC server.
// It contains an HTTP server that listens for incoming RPC requests.
type RPCServer struct {
	server *http.Server
}

// Start starts the RPC server and begins listening for incoming requests.
func (s *RPCServer) Start() error {
	return s.server.ListenAndServe()
}

// Stop stops the RPC server by closing the underlying HTTP server.
func (s *RPCServer) Stop() error {
	return s.server.Close()
}

// NewRPCHandler creates a new RPC server with the given configuration.
// It initializes the HTTP server and registers the CommandHandler service.
// The CommandHandler service provides methods for changing log levels and sending commands to services.
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

// CommandHandler is a struct that implements the RPC service for handling commands.
type CommandHandler struct {
	sLogger log.Logger // service logger
	iLogger log.Logger // internal logger
}

// ChangeLogLevel changes the log level for the service and internal logger.
func (h CommandHandler) ChangeLogLevel(level log.Level, _ *error) error {
	h.sLogger.SetLevel(level)
	h.iLogger.SetLevel(level)
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
