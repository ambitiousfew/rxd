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

	s.log.Info("server starting", "address", s.server.Addr)
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return errors.New("server shutdown: " + err.Error())
	}

	<-doneC // wait for signal routine to finish...
	return nil
}

func (s *HelloWorldAPIService) Init(ctx rxd.ServiceContext) error {
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

func (s *HelloWorldAPIService) Idle(ctx rxd.ServiceContext) error {
	s.log.Info("entering idle")
	return nil
}

func (s *HelloWorldAPIService) Stop(ctx rxd.ServiceContext) error {
	s.log.Info("entering stop")
	s.server = nil
	return nil
}

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(logHandler).With("service", "hello-world-example")

	// We create an instance of our service
	helloWorld := NewHelloWorldService()
	helloWorld.log = logger

	serviceOpts := []rxd.ServiceOption{
		rxd.UsingRunPolicy(rxd.PolicyContinue),
	}

	apiSvc := rxd.NewService("HelloWorldAPI", helloWorld, serviceOpts...)

	dopts := []rxd.DaemonOption{
		// rxd.UsingLogger(logger),
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
