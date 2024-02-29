package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd"
)

var _ rxd.Service = &HelloWorldAPIService{}

const serviceName = "HelloWorldAPI"

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

func (s *HelloWorldAPIService) Setup(ctx context.Context) rxd.ServiceConfig {
	return rxd.ServiceConfig{
		Name: serviceName,
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(ctx context.Context) rxd.ServiceResponse {
	go func() {
		<-ctx.Done() // wait for shutdown signal against this service

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := s.server.Shutdown(timeoutCtx)
		if err != nil {
			s.
		}
	}()

	// sc.Log.Info("server starting", "service", serviceName, "address", s.server.Addr)
	log.Println("server starting", "service", serviceName, "address", s.server.Addr)
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.Idle)
	}

	log.Println("server shutdown", "service", serviceName)

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly

	// Run moves to StopState when the server is shutdown
	return rxd.NewResponse(nil, rxd.Stop)
}

func (s *HelloWorldAPIService) Init(ctx context.Context) rxd.ServiceResponse {
	// always respect the context Done signal in case shutdown is taking place.
	// without it, you ignore shutdown and continue to change states.
	// choosing to ignore the Done signal would result in daemon.Start() never returning.
	// All services must reach the Exit state before daemon.Start() will return.
	select {
	case <-ctx.Done():
		return rxd.NewResponse(nil, rxd.Exit)
	default:
		// Init moves to Idle
		return rxd.NewResponse(nil, rxd.Run)
	}
}

func (s *HelloWorldAPIService) Idle(ctx context.Context) rxd.ServiceResponse {
	log.Println("idle state")
	select {
	case <-ctx.Done():
		return rxd.NewResponse(nil, rxd.Exit)
	default:
		// Idle moves to Run
		return rxd.NewResponse(nil, rxd.Run)
	}
}

func (s *HelloWorldAPIService) Stop(ctx context.Context) rxd.ServiceResponse {
	// Stop provides an opportunity to perform any cleanups that should be done before the service is exited or re-initialized.
	// State changes to Exit will always run Stop first.
	log.Println("stopping server")
	select {
	case <-ctx.Done():
		return rxd.NewResponse(nil, rxd.Exit)
	default:
		// Stop could move to Exit or even back to Init
		// Reaching Exit will cause the service to be in an Exited state.
		return rxd.NewResponse(nil, rxd.Exit)
	}
}

var _ rxd.Service = &HelloWorldAPIService{}

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// We create an instance of our service
	helloWorld := NewHelloWorldService()

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(rxd.DaemonConfig{
		Name:    "hello-world-daemon",
		Signals: []os.Signal{os.Interrupt, syscall.SIGINT, syscall.SIGTERM},
	})

	err = daemon.AddService(helloWorld)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// We can set the log severity we want to observe, LevelInfo is default
	err := daemon.Start(ctx) // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
