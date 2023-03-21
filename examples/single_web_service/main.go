package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

// HelloWorldAPIService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type HelloWorldAPIService struct {
	// cfg can be named anything but it MUST exist as *rxdaemon.ServiceConfig, Config() method will return it.
	cfg *rxd.ServiceConfig

	// fields this specific server uses
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService(cfg *rxd.ServiceConfig) *HelloWorldAPIService {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"hello": "world"}`))
	})

	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	return &HelloWorldAPIService{
		cfg:    cfg,
		server: server,

		ctx:    ctx,
		cancel: cancel,
	}
}

// Name give your service a log friendly name
func (s *HelloWorldAPIService) Name() string {
	return "HelloWorldAPI"
}

// Config should always return the ServiceConfig instance stored in the service struct.
// The rxdaemon manager needs this to access things like the service shutdown channel
func (s *HelloWorldAPIService) Config() *rxd.ServiceConfig {
	return s.cfg
}

// Init can be used to do any preparation that maybe doesnt belong in instance creation
// but sometime between instance creation and before you start your pre-checks to run.
func (s *HelloWorldAPIService) Init() rxd.ServiceResponse {
	// if all is well here, move to the next state or skip to RunState
	return rxd.NewResponse(nil, rxd.IdleState)
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *HelloWorldAPIService) Idle() rxd.ServiceResponse {
	// if all is well here, move to the RunState or retry back to Init if something went wrong.
	return rxd.NewResponse(nil, rxd.RunState)
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run() rxd.ServiceResponse {

	go func() {
		// We should always watch for this signal, must use goroutine here
		// since ListenAndServe will block and we need a way to end the
		// server as well as inform the server to stop all requests ASAP.
		<-s.cfg.ShutdownC
		s.cfg.LogInfo(fmt.Sprintf("%s received a shutdown signal, cancel server context to stop server gracefully", s.Name()))
		s.cancel()
		s.server.Shutdown(s.ctx)
	}()

	s.cfg.LogInfo(fmt.Sprintf("%s server starting at %s", s.Name(), s.server.Addr))
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.IdleState)
	}

	s.cfg.LogInfo(fmt.Sprintf("%s server shutdown", s.Name()))

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly
	return rxd.NewResponse(nil, rxd.StopState)
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *HelloWorldAPIService) Stop() rxd.ServiceResponse {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	return rxd.NewResponse(nil, rxd.ExitState)
}

// This line is purely for error checking to ensure we are meeting the Service interface.
var _ rxd.Service = &HelloWorldAPIService{}

// Example entrypoint
func main() {
	// We create an instance of our ServiceConfig
	apiCfg := rxd.NewServiceConfig(
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
	)
	// We create an instance of our service
	apiSvc := NewHelloWorldService(apiCfg)

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(apiSvc)
	// We can set the log severity we want to observe, LevelInfo is default
	daemon.SetLogSeverity(rxd.LevelInfo)

	err := daemon.Start() // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
