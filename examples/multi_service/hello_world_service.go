package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ambitiousfew/rxd"
)

type HelloWorldAPIService struct {
	// fields this specific server uses
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() *HelloWorldAPIService {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"hello": "world"}`))
	})

	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	return &HelloWorldAPIService{
		server: server,

		ctx:    ctx,
		cancel: cancel,
	}
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *HelloWorldAPIService) Idle(cfg *rxd.ServiceConfig) rxd.ServiceResponse {
	// if all is well here, move to the RunState or retry back to Init if something went wrong.
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()

	cfg.LogInfo(fmt.Sprintf("intentionally delaying for 20s before run begins."))
	for {
		select {
		case <-cfg.ShutdownC:
			return rxd.NewResponse(nil, rxd.StopState)
		case <-timer.C:
			// Intentional 20s delay so polling service can react to failed attempts to this API.
			return rxd.NewResponse(nil, rxd.RunState)
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(cfg *rxd.ServiceConfig) rxd.ServiceResponse {

	go func() {
		// We should always watch for this signal, must use goroutine here
		// since ListenAndServe will block and we need a way to end the
		// server as well as inform the server to stop all requests ASAP.
		<-cfg.ShutdownC
		cfg.LogInfo(fmt.Sprintf("received a shutdown signal, cancel server context to stop server gracefully"))
		s.cancel()
		s.server.Shutdown(s.ctx)
	}()

	// We have made it to Run() of Hello World, we can notify our dependent service (Poll Service) that we made it here.
	// you can pass any state you want to notify, it makes the most sense to pass the state you are currently on.
	// But you could pass the state you wish the other service to enter as well.
	cfg.NotifyStateChange(rxd.RunState)

	cfg.LogInfo(fmt.Sprintf("server starting at %s", s.server.Addr))
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.IdleState)
	}

	cfg.LogInfo(fmt.Sprintf("server shutdown"))

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly
	return rxd.NewResponse(nil, rxd.StopState)
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *HelloWorldAPIService) Stop(cfg *rxd.ServiceConfig) rxd.ServiceResponse {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	cfg.LogInfo(fmt.Sprintf("is stopping"))
	return rxd.NewResponse(nil, rxd.ExitState)
}
