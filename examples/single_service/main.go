// For this example we will create a simple API service that will run until it is stopped
// either via the context timeout (30s) or an OS signal (SIGINT, SIGTERM).
// It runs a Hello World API server on port 8000, http://127.0.0.1:8000/
//
// During run the service spawns an intentional timeout error that triggers a shutdown of the server.
// This is just to simulate edge cases that might lead to something forcing an error against the
// HTTP server causing it to shutdown and to show how the service will handle cycling back through
// the lifecycle methods.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

const DaemonName = "single-service"

// HelloWorldAPIService must meet Service interface or line below errors.
var _ rxd.ServiceRunner = (*HelloWorldAPIService)(nil)

// Example entrypoint
func main() {

	// NOTE: Intentional cancellation timeout after 30 seconds to show startup/shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// create a new default logger
	logger := log.NewLogger(log.LevelInfo, log.NewHandler())

	// Create a new instance of your service must meet the ServiceRunner interface.
	helloWorld := NewHelloWorldService()

	// Give the service a name, the service runner, and any options you want to pass to the service.
	apiSvc := rxd.NewService("helloworld-api", helloWorld)

	// This is only used for the commented out PrestartPipeline below
	ready := &atomic.Bool{}
	timer := time.NewTimer(6 * time.Second)
	go func() {
		<-timer.C
		timer.Stop()
		ready.Store(true)
	}()

	prestartConf := rxd.PrestartConfig{
		RestartOnError: true,             // if any stage errors, restart the pipeline from the beginning
		RestartDelay:   10 * time.Second, // delay between restarts
	}

	prestartStages := []rxd.Stage{
		{
			// some arbitrary stage to demonastrate how to use the prestart stages
			Name: "timer-ready",
			Func: func(ctx context.Context) error {
				if ready.Load() {
					return nil
				}
				return errors.New("timer is not ready")
			},
		},
	}

	// customizing daemon options
	dopts := []rxd.DaemonOption{
		rxd.WithSignals(os.Interrupt, syscall.SIGINT, syscall.SIGTERM),
		// This adds a prestart pipeline that will run the stages (if any) in order and restart from the beginning if an error occurs.
		rxd.WithPrestart(prestartConf, prestartStages...),
		// This adds a service logger to the daemon so all services can log in the same logging format.
		rxd.WithServiceLogger(logger),
	}

	// Create a new daemon giving it a name, service logger and options
	daemon := rxd.NewDaemon(DaemonName, dopts...)

	// Add the single service to the daemon
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

func (s *HelloWorldAPIService) Run(sctx rxd.ServiceContext) error {
	// Perform your main service work here. This is a good place to start serving requests, processing data, etc.
	// NOTE: RxD default Manager will not allow two lifecycles to run at the same time.
	// So we must launch a goroutine to to watch and signal the server shutdown in this case.
	doneC := make(chan struct{})
	go func(server *http.Server) {
		defer close(doneC)

		// NOTE: Intentional cancellation timeout to showcase how the lifecycles work.
		errTimeout := time.NewTimer(7 * time.Second)
		defer errTimeout.Stop()

		select {
		case <-sctx.Done():
			return
		case <-errTimeout.C:
			sctx.Log(log.LevelError, "timeout waiting for context to close")
		}

		// NOTE: because Stop and Run cannot execute at the same time, we need to stop the server here
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(timeoutCtx); err != nil {
			sctx.Log(log.LevelError, "error shutting down server: "+err.Error())
		} else {
			sctx.Log(log.LevelInfo, "server shutdown")
		}
	}(s.server)

	sctx.Log(log.LevelInfo, "server starting with address: "+s.server.Addr)
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return errors.New("server shutdown: " + err.Error())
	}

	<-doneC // wait for signal routine to finish...
	sctx.Log(log.LevelInfo, "server stopped")
	return rxd.ErrLifecycleDone // returning this error wont be printed but will take the same error path.
}

func (s *HelloWorldAPIService) Init(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelInfo, "entering init")
	// Perform any initialization work here. This is a good place to create connections, open files, etc.
	if s.server != nil {
		sctx.Log(log.LevelInfo, "server already initialized")
		return errors.New("server already initialized")
	}

	// we can reinitialize our mux and reassign the server since its unusable after shutdown.
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

func (s *HelloWorldAPIService) Idle(sctx rxd.ServiceContext) error {
	// Perform any idling work here. This is a good place to wait for signals or other events.
	// Retry connections, wait for other services to enter a given state, etc.
	sctx.Log(log.LevelInfo, "entering idle")
	select {
	case <-sctx.Done():
		return nil
	default:
		return nil
	}
}

func (s *HelloWorldAPIService) Stop(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelInfo, "entering stop")
	// perform any cleanup that idle or run may have created.
	s.server = nil
	select {
	case <-sctx.Done():
		return nil
	default:
		return nil
	}
}
