package rxd

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ambitiousfew/rxd/v2/intracom"
	"github.com/ambitiousfew/rxd/v2/log"
	"github.com/ambitiousfew/rxd/v2/pkg/rpc"
)

type RPCConfig struct {
	Addr string
	Port uint16
}

type CommandHandlerConfig struct {
	Services map[string]chan rpc.CommandSignal
	// Intracom *intracom.Intracom
	CommandRequestTopic intracom.Topic[RPCCommandRequest]
	// ResponseTopic intracom.Topic[RPCCommandRequest]
	Logger log.Logger
}

func NewCommandHandler(config CommandHandlerConfig) (RPCCommandHandler, error) {
	if config.Services == nil {
		return RPCCommandHandler{}, errors.New("services not set")
	}

	if config.Logger == nil {
		return RPCCommandHandler{}, errors.New("logger not set")
	}

	if config.CommandRequestTopic == nil {
		return RPCCommandHandler{}, errors.New("command request topic not set")
	}

	return RPCCommandHandler{
		sLogger:  config.Logger,
		services: config.Services,
		// ic:       config.Intracom,
		requestTopic: config.CommandRequestTopic,

		stopped: &atomic.Bool{},
		stopC:   make(chan struct{}),
	}, nil
}

type RPCCommandHandler struct {
	sLogger      log.Logger                        // service logger
	services     map[string]chan rpc.CommandSignal // valid services that can be controlled via command signals
	requestTopic intracom.Topic[RPCCommandRequest]

	// respTopic intracom.Topic[RPCCommandRequest]
	ic      *intracom.Intracom
	stopped *atomic.Bool
	stopC   chan struct{}
}

func (h RPCCommandHandler) ChangeLogLevel(level log.Level, resp *error) error {
	if h.sLogger == nil {
		err := errors.New("service logger not set, cannot change log level")
		*resp = err
		return err
	}

	h.sLogger.SetLevel(level)
	return nil
}

func (h RPCCommandHandler) SendCommandSignal(payload rpc.CommandSignalPayload, resp *error) error {
	var err error
	if h.stopped.Load() {
		err = errors.New("RPC Handler has been stopped")
		*resp = err
		return err
	}

	if payload.Signal == rpc.CommandSignalUnknown {
		err = errors.New("unknown command signal '" + string(payload.Signal) + "'")
		*resp = err
		return err
	}

	// send is intended for all services
	if payload.Service == "--all" {
		// 100 ms for each service to respond.
		timer := time.NewTimer(100 * time.Millisecond)
		defer timer.Stop()

		unresponsives := make([]string, 0)

		for service, relayC := range h.services {
			select {
			// block until a case matches.
			case <-h.stopC:
				// if handler is stopped while we are waiting to send
				err = errors.New("RPC Handler has been stopped")
				*resp = err
				return err
			case <-timer.C:
				// if we have waited for 100 ms and the service has not responded
				unresponsives = append(unresponsives, service)
			case relayC <- payload.Signal:
				// service took the command signal
			}

			// reset ticker move to the next service
			timer.Reset(100 * time.Millisecond)
		}

		if len(unresponsives) == len(h.services) {
			err = errors.New("no services responded in time")
			*resp = err
			return err
		}

		if len(unresponsives) > 0 {
			err = errors.New("some services did not respond in time: " + strings.Join(unresponsives, ", "))
			*resp = err
			return err
		}

		return nil
	}

	// the send is intendent for a specific service
	relayC, found := h.services[payload.Service]
	if !found {
		err = errors.New("service not found: " + payload.Service)
		*resp = err
		return err
	}

	select {
	case <-h.stopC:
		// if handler is stopped while we are waiting to send
		err = errors.New("RPC Handler has been stopped")
		*resp = err
		return err
	case relayC <- payload.Signal:
	}
	return nil
}

func (h RPCCommandHandler) start(ctx context.Context) error {
	h.stopped.Store(false)

	subConfig := intracom.SubscriberConfig[RPCCommandRequest]{
		ConsumerGroup: "rpc-command-handler",
		ErrIfExists:   false,
		BufferSize:    1,
		BufferPolicy:  intracom.BufferPolicyDropNone[RPCCommandRequest]{},
	}

	sub, err := h.requestTopic.Subscribe(ctx, subConfig)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-h.stopC:
			h.stopped.Store(true)
			return nil
		case cmdReq, open := <-sub:
			if !open {
				return nil
			}

			switch cmdReq.Service {
			case "_all":
				// handle all services
			default:
				// handle specific service
				cmdRelayC, found := h.services[cmdReq.Service]
				if !found {
					// we dont know about that service
					// send an error back to the client
					continue
				}

				switch cmdReq.Signal {
				case rpc.CommandSignalUnknown:
					// unknown signal
					continue
				default:
				}

				// send the command to the service manager
				select {
				case <-ctx.Done():
					return nil
				case cmdRelayC <- cmdReq.Signal:
				}

			}

		}
	}

}

func (h RPCCommandHandler) close() error {
	if h.stopped.Load() {
		return nil
	}

	close(h.stopC)
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
