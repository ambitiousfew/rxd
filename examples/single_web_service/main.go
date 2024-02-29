package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

// HelloWorldAPIService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type HelloWorldAPIService struct {
	// fields this specific server uses
	server *http.Server
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() *HelloWorldAPIService {
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
		server: server,
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(sc *rxd.ServiceContext) rxd.ServiceResponse {
	go func() {
		<-sc.ShutdownCtx.Done() // wait for shutdown signal against this service
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}()

	sc.Log.Info("server starting", "service", sc.Name, "address", s.server.Addr)
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.IdleState)
	}

	sc.Log.Info("server shutdown")

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly

	// Run moves to StopState when the server is shutdown
	return rxd.NewResponse(nil, rxd.StopState)
}

func (s *HelloWorldAPIService) Init(sc *rxd.ServiceContext) rxd.ServiceResponse {
	// Init moves to IdleState
	return rxd.NewResponse(nil, rxd.IdleState)
}

func (s *HelloWorldAPIService) Idle(sc *rxd.ServiceContext) rxd.ServiceResponse {
	// Idle moves to RunState
	sc.Log.Info("idle state")
	return rxd.NewResponse(nil, rxd.RunState)
}

func (s *HelloWorldAPIService) Stop(sc *rxd.ServiceContext) rxd.ServiceResponse {
	sc.Log.Info("stopping server")
	// Stop moves to ExitState
	return rxd.NewResponse(nil, rxd.ExitState)
}

// HelloWorldAPIService must meet Service interface or line below errors.
var _ rxd.Service = &HelloWorldAPIService{}

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// We create an instance of our service
	helloWorld := NewHelloWorldService()
	// We create an instance of our ServiceConfig
	apiOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy))

	apiSvc := rxd.NewService("HelloWorldAPI", helloWorld, apiOpts)

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)
	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(rxd.DaemonConfig{
		Name:               "hello-world-example",
		LogHandler:         handler,
		IntracomLogHandler: handler,
		Signals:            []os.Signal{os.Interrupt, os.Kill},
	})

	daemon.AddService(apiSvc)

	// We can set the log severity we want to observe, LevelInfo is default

	err := daemon.Start(ctx) // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	logger.Info("successfully stopped daemon")
}
