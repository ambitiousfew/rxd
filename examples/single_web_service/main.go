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
	// fields this specific server uses
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() *HelloWorldAPIService {
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
		server: server,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(c *rxd.ServiceContext) rxd.ServiceResponse {
	go func() {
		// We should always watch for this signal, must use goroutine here
		// since ListenAndServe will block and we need a way to end the
		// server as well as inform the server to stop all requests ASAP.
		<-c.ShutdownSignal()
		c.LogInfo(fmt.Sprintf("received a shutdown signal, cancel server context to stop server gracefully"))
		s.cancel()
		s.server.Shutdown(s.ctx)
	}()

	c.LogInfo(fmt.Sprintf("server starting at %s", s.server.Addr))
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		// Stop running, move back to an Idle retry state
		return rxd.NewResponse(err, rxd.IdleState)
	}

	c.LogInfo(fmt.Sprintf("server shutdown"))

	// If we reached this point, we stopped the server without erroring, we are likely trying to stop our daemon.
	// Lets stop this service properly
	return rxd.NewResponse(nil, rxd.StopState)
}

// Example entrypoint
func main() {
	// We create an instance of our service
	helloWorld := NewHelloWorldService()

	// We create an instance of our ServiceConfig
	apiOpts := rxd.NewServiceOpts(
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
	)

	apiSvc := rxd.NewService("HelloWorldAPI", apiOpts)
	apiSvc.UsingRunStage(helloWorld.Run)

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(apiSvc)
	// We can set the log severity we want to observe, LevelInfo is default

	err := daemon.Start() // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
