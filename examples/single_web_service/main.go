package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/log/standard"
)

const DaemonName = "single-service"

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// A logger is required. You can use the default logger or pass your own that meets log.ger interface.
	// logger := rxd.NewDefaultLogger(log.LevelDebug)
	logger := standard.NewDefaultLogger(log.LevelDebug)
	// logger := journald.New(log.LevelDebug)

	// daemon options
	dopts := []rxd.DaemonOption{}
	// Create a new daemon instance with a name and options
	daemon := rxd.NewDaemon(DaemonName, logger, dopts...)

	// We create an instance of our service
	helloWorld := NewHelloWorldService()

	// NOTE:
	// 1. Dont have to protect against VALUE structs with pointer receivers because the compiler will catch it.
	//

	// var handler1 *MyHandler // is a pointer type, will run when receiver is a pointer
	var handler2 MyHandler // is a pointer type, will not run when receiver uses a dereferenced value.
	// handler3 := MyHandler{} // not a pointer
	apiSvc := rxd.NewService("helloworld-api", helloWorld, rxd.WithHandler(handler2))

	err := daemon.AddService(apiSvc)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}

	// Blocks main thread, runs added services in their own goroutines
	err = daemon.Start(ctx)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}

	logger.Log(log.LevelInfo, "successfully stopped daemon")
}

// HelloWorldAPIService is a simple service that runs a web server that returns a JSON response.

// HelloWorldAPIService must meet Service interface or line below errors.
var _ rxd.ServiceRunner = (*HelloWorldAPIService)(nil)

// var *MyHandler must meet ServiceHandler interface or line below errors.
type MyHandler struct{}

func (h MyHandler) Handle(sctx rxd.ServiceContext, ds rxd.DaemonService, updateState chan<- rxd.StateUpdate) {

	state := rxd.StateInit

	timeout := time.NewTicker(1 * time.Second)
	defer timeout.Stop()
	// Set the default timeout to 0 to default resets on everything else except stop.
	for state != rxd.StateExit {
		var err error
		updateState <- rxd.StateUpdate{State: state, Name: ds.Name}
		select {
		case <-sctx.Done():
			state = rxd.StateExit
		case <-timeout.C:
		}

		switch state {
		case rxd.StateInit:

			state, err = ds.Runner.Init(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
				state = rxd.StateExit
			}
		case rxd.StateIdle:

			state, err = ds.Runner.Idle(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
				state = rxd.StateStop
			}
		case rxd.StateRun:

			state, err = ds.Runner.Run(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
		case rxd.StateStop:
			if state, err = ds.Runner.Stop(sctx); err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
		}
		timeout.Reset(1 * time.Second)
	}
}

// HelloWorldAPIService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
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

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(ctx rxd.ServiceContext) (rxd.State, error) {
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		// DEBUG: we are trying to make the server shutdown to see if it will stop the service to check policy
		errTimeout := time.NewTimer(7 * time.Second)
		defer errTimeout.Stop()

		select {
		case <-ctx.Done():
		case <-errTimeout.C:
			ctx.Log(log.LevelError, "timeout waiting for context to close")
		}

		// NOTE: because Stop and Run cannot execute at the same time, we need to stop the server here
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.server.Shutdown(timeoutCtx); err != nil {
			ctx.Log(log.LevelError, "error shutting down server: "+err.Error())
		} else {
			ctx.Log(log.LevelInfo, "server shutdown")
		}
	}()

	ctx.Log(log.LevelInfo, "server starting with address: "+s.server.Addr)
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return rxd.StateStop, errors.New("server shutdown: " + err.Error())
	}

	<-doneC // wait for signal routine to finish...
	return rxd.StateStop, nil
}

func (s *HelloWorldAPIService) Init(ctx rxd.ServiceContext) (rxd.State, error) {
	// handle initializing the primary focus of what the service will run.
	// Stop will be responsible for cleaning up any resources that were created during Init.
	if s.server != nil {
		ctx.Log(log.LevelInfo, "server already initialized")
		return rxd.StateStop, errors.New("server already initialized")
	}

	ctx.Log(log.LevelInfo, "entering init")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"hello": "world"}`))
	})

	s.server = &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	return rxd.StateIdle, nil
}

func (s *HelloWorldAPIService) Idle(ctx rxd.ServiceContext) (rxd.State, error) {
	ctx.Log(log.LevelInfo, "entering idle")
	return rxd.StateRun, nil
}

func (s *HelloWorldAPIService) Stop(ctx rxd.ServiceContext) (rxd.State, error) {
	ctx.Log(log.LevelInfo, "entering stop")
	s.server = nil
	return rxd.StateExit, nil
}
