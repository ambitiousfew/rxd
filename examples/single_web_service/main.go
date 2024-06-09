package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

// HelloWorldAPIService must meet Service interface or line below errors.
var _ rxd.ServiceRunner = (*HelloWorldAPIService)(nil)

// HelloWorldAPIService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type HelloWorldAPIService struct {
	// fields this specific server uses
	server *http.Server
	log    *slog.Logger
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() *HelloWorldAPIService {
	return &HelloWorldAPIService{
		server: nil,
		log:    slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(ctx context.Context) error {
	go func() {

		// DEBUG: we are trying to make the server shutdown to see if it will stop the service to check policy
		errTimeout := time.NewTimer(7 * time.Second)
		defer errTimeout.Stop()

		select {
		case <-ctx.Done():
		case <-errTimeout.C:
			s.log.Info("timer has elapsed, forcing server shutdown")
		}

		// NOTE: because Stop and Run cannot execute at the same time, we need to stop the server here
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.log.Error("error shutting down server", "error", err)
		} else {
			s.log.Info("server shutdown")
		}
	}()

	// sc.Log.Info("server starting", "service", sc.Name, "address", s.server.Addr)
	s.log.Info("server starting", "address", s.server.Addr)
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return errors.New("server shutdown")
	}

	// sc.Log.Info("server shutdown")

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly

	// Run moves to StopState when the server is shutdown
	return nil
}

func (s *HelloWorldAPIService) Init(ctx context.Context) error {
	// handle initializing the primary focus of what the service will run.
	// Stop will be responsible for cleaning up any resources that were created during Init.
	if s.server != nil {
		s.log.Error("server already initialized")
		return errors.New("server already initialized")
	}

	s.log.Info("entering init")
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

func (s *HelloWorldAPIService) Idle(ctx context.Context) error {
	s.log.Info("entering idle")
	return nil
}

func (s *HelloWorldAPIService) Stop(ctx context.Context) error {
	s.log.Info("entering stop")
	s.server = nil
	return nil
}

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler).With("service", "hello-world-example")

	// We create an instance of our service
	helloWorld := NewHelloWorldService()
	helloWorld.log = logger

	// We create an instance of our ServiceConfig
	opts := []rxd.ServiceOption{
		// rxd.UsingRunPolicy(rxd.RunUntilSignaled),
		// rxd.UsingRunPolicyConfig(rxd.RunPolicyConfig{
		// 	RestartDelay: 10 * time.Second,
		// }),

		rxd.UsingPolicyHandler(rxd.GetPolicyHandler(rxd.RunPolicyConfig{
			Policy:       rxd.PolicyRunContinous,
			RestartDelay: 10 * time.Second,
		})),

		// NOTE: Users can inject their own policy now if they want to.
		// rxd.UsingPolicyHandler(rxd.GetPolicyHandler(rxd.PolicyRunContinous)),
	}

	apiSvc := rxd.NewService("HelloWorldAPI", helloWorld, opts...)

	dopts := []rxd.DaemonOption{
		rxd.UsingLogger(logger),
	}

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon("hello-world-example", dopts...)

	err := daemon.AddService(apiSvc)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	err = daemon.Start(ctx) // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	logger.Info("successfully stopped daemon")
}
