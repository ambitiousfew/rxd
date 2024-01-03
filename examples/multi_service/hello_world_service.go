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
	ctx, cancel := context.WithCancel(context.Background())

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
func (s *HelloWorldAPIService) Idle(c *rxd.ServiceContext) rxd.ServiceResponse {
	// if all is well here, move to the RunState or retry back to Init if something went wrong.
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()

	c.Log.Info("intentionally delaying for 8s before run begins")
	for {
		select {
		case <-c.ShutdownCtx.Done():
			return rxd.NewResponse(nil, rxd.StopState)
		case <-timer.C:
			// Intentional 20s delay so polling service can react to failed attempts to this API.
			return rxd.NewResponse(nil, rxd.RunState)
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(c *rxd.ServiceContext) rxd.ServiceResponse {

	go func() {
		// We should always watch for this signal, must use goroutine here
		// since ListenAndServe will block and we need a way to end the
		// server as well as inform the server to stop all requests ASAP.
		<-c.ShutdownCtx.Done()
		c.Log.Info("received a shutdown signal, cancel server context to stop server gracefully")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}()

	c.Log.Info(fmt.Sprintf("server starting at %s", s.server.Addr))
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.IdleState)
	}

	c.Log.Info("server shutdown")

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly
	return rxd.NewResponse(nil, rxd.StopState)
}

func (s *HelloWorldAPIService) Init(c *rxd.ServiceContext) rxd.ServiceResponse {
	c.Log.Info("initializing")
	return rxd.NewResponse(nil, rxd.IdleState)
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *HelloWorldAPIService) Stop(c *rxd.ServiceContext) rxd.ServiceResponse {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	c.Log.Info("stopping")
	return rxd.NewResponse(nil, rxd.ExitState)
}

// Ensure we meet the interface or error.
var _ rxd.Service = &HelloWorldAPIService{}
