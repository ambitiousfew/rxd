package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

// Ensure we meet the interface or error.
var _ rxd.ServiceRunner = (*HelloWorldAPIService)(nil)

type HelloWorldAPIService struct {
	// fields this specific server uses
	server *http.Server
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() *HelloWorldAPIService {
	return &HelloWorldAPIService{
		server: nil,
	}
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *HelloWorldAPIService) Idle(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelDebug, "entered idle state")
	// if all is well here, move to the RunState or retry back to Init if something went wrong.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	// sctx.Log(log.LevelInfo, "intentionally delaying for 8s before run begi)
	for {
		select {
		case <-sctx.Done():
			return nil
		case <-timer.C:
			// Intentional 20s delay so polling service can react to failed attempts to this API.
			return nil
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelDebug, "entered run state")
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		// We should always watch for this signal, must use goroutine here
		// since ListenAndServe will block and we need a way to end the
		// server as well as inform the server to stop all requests ASAP.
		<-sctx.Done()
		sctx.Log(log.LevelWarning, "received a shutdown signal, cancel server context to stop server gracefully")

		timedCtx, timedCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer timedCancel()

		err := s.server.Shutdown(timedCtx)
		if err != nil {
			sctx.Log(log.LevelError, err.Error())
		}
	}()

	sctx.Log(log.LevelInfo, fmt.Sprintf("server starting at %s", s.server.Addr))
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return err
	}

	<-doneC
	sctx.Log(log.LevelInfo, "server shutdown")

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly
	return nil
}

func (s *HelloWorldAPIService) Init(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelDebug, "entered init state")

	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"hello": "world"}`))
	})

	s.server = &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	return nil
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *HelloWorldAPIService) Stop(sctx rxd.ServiceContext) error {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	sctx.Log(log.LevelDebug, "entered init state")
	s.server = nil

	select {
	case <-sctx.Done():
		return nil
	default:
		return nil
	}
}
