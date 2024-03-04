package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

type HelloWorldAPIService struct {
	// fields this specific server uses
	server *http.Server
	log    *slog.Logger
}

func usingSlogLogger(l *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)
			l.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", duration.Milliseconds())
		})
	}
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() rxd.Service {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("service", HelloWorldAPI)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"hello": "world"}`))
	})

	log := log.New(os.Stderr, HelloWorldAPI, log.LstdFlags)

	// Add middleware to log requests
	loggerMux := usingSlogLogger(logger)(mux)

	server := &http.Server{
		Addr:     ":8000",
		Handler:  loggerMux,
		ErrorLog: log,
	}

	s := &HelloWorldAPIService{
		server: server,
		log:    logger,
	}

	return rxd.Service{
		Conf: rxd.ServiceConfig{
			Name: HelloWorldAPI,
		},
		Svc: s,
	}
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *HelloWorldAPIService) Idle(sc rxd.ServiceContext) rxd.ServiceResponse {
	// if all is well here, move to the RunState or retry back to Init if something went wrong.
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()

	// s.log.Info("intentionally delaying for 8s before run begins")
	for {
		select {
		case <-sc.Done():
			return rxd.NewResponse(nil, rxd.Stop)
		case <-timer.C:
			// Intentional 8s delay so polling service can react to failed attempts to this API.
			return rxd.NewResponse(nil, rxd.Run)
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(sc rxd.ServiceContext) rxd.ServiceResponse {

	go func() {
		// We should always watch for this signal, must use goroutine here
		// since ListenAndServe will block and we need a way to end the
		// server as well as inform the server to stop all requests ASAP.
		<-sc.Done()
		s.log.Info("received a shutdown signal, cancel server context to stop server gracefully")
		ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := s.server.Shutdown(ctxTimeout)
		if err != nil {
			s.log.Error("server shutdown error", "error", err)
		}
	}()

	s.log.Info(fmt.Sprintf("server starting at %s", s.server.Addr))
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.Idle)
	}

	s.log.Info("server shutdown")

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly
	return rxd.NewResponse(nil, rxd.Stop)
}

func (s *HelloWorldAPIService) Init(sc rxd.ServiceContext) rxd.ServiceResponse {
	s.log.Info("initializing")
	return rxd.NewResponse(nil, rxd.Idle)
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *HelloWorldAPIService) Stop(sc rxd.ServiceContext) rxd.ServiceResponse {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	s.log.Info("stopping")
	return rxd.NewResponse(nil, rxd.Exit)
}

// Ensure we meet the interface or error.
var _ rxd.Servicer = &HelloWorldAPIService{}
