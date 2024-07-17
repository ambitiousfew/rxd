package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/log/journald"
)

const DaemonName = "single-service"

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// A logger is required. You can use the default logger or pass your own that meets log.ger interface.

	handler := journald.NewHandler()
	logger := log.NewLogger(log.LevelInfo, handler)

	// Create a new "instance" of our type that meets the interface for a ServiceRunner.
	helloWorld := NewHelloWorldService() // Service runner

	var manager2 MyManager // using custom defined handler
	// Give the service a name, the service runner, and any options you want to pass to the service.
	apiSvc := rxd.NewService("helloworld-api", helloWorld, rxd.WithManager(manager2))

	// daemon options
	dopts := []rxd.DaemonOption{}
	// Create a new daemon instance with a name and options
	daemon := rxd.NewDaemon(DaemonName, logger, dopts...)

	// Add the service to the daemon
	err := daemon.AddService(apiSvc)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}

	// Start the daemon, this will block until the daemon is stopped via ctx cancel or OS signal.
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

// var *MyManager must meet ServiceManager interface or line below errors.
type MyManager struct{}

func (m MyManager) Manage(sctx rxd.ServiceContext, ds rxd.DaemonService, updateState func(string, rxd.State)) {

	state := rxd.StateInit

	timeout := time.NewTicker(1 * time.Second)
	defer timeout.Stop()
	// Set the default timeout to 0 to default resets on everything else except stop.
	for state != rxd.StateExit {

		updateState(ds.Name, state)

		select {
		case <-sctx.Done():
			state = rxd.StateExit
		case <-timeout.C:
		}

		var err error
		switch state {
		case rxd.StateInit:
			err = ds.Runner.Init(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
				state = rxd.StateExit
			} else {
				state = rxd.StateIdle
			}
		case rxd.StateIdle:
			err = ds.Runner.Idle(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
				state = rxd.StateStop
			} else {
				state = rxd.StateRun
			}
		case rxd.StateRun:
			err = ds.Runner.Run(sctx)
			if err != nil {
				sctx.Log(log.LevelError, err.Error())
			}
			state = rxd.StateStop
		case rxd.StateStop:
			if err = ds.Runner.Stop(sctx); err != nil {
				sctx.Log(log.LevelError, err.Error())
				state = rxd.StateExit
			} else {
				state = rxd.StateInit
			}

		}
		timeout.Reset(1 * time.Second)
	}

	updateState(ds.Name, rxd.StateExit)
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
func (s *HelloWorldAPIService) Run(ctx rxd.ServiceContext) error {
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
		return errors.New("server shutdown: " + err.Error())
	}

	<-doneC // wait for signal routine to finish...
	ctx.Log(log.LevelInfo, "server stopped")
	return nil
}

func (s *HelloWorldAPIService) Init(ctx rxd.ServiceContext) error {
	// handle initializing the primary focus of what the service will run.
	// Stop will be responsible for cleaning up any resources that were created during Init.
	if s.server != nil {
		ctx.Log(log.LevelInfo, "server already initialized")
		return errors.New("server already initialized")
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
	return nil
}

func (s *HelloWorldAPIService) Idle(ctx rxd.ServiceContext) error {
	ctx.Log(log.LevelInfo, "entering idle")
	return nil
}

func (s *HelloWorldAPIService) Stop(ctx rxd.ServiceContext) error {
	ctx.Log(log.LevelInfo, "entering stop")
	s.server = nil
	return nil
}
