package main

import (
	"log"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

type SimpleService struct{}

func NewSimpleService() *SimpleService {
	return &SimpleService{}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *SimpleService) Run(c *rxd.ServiceContext) rxd.ServiceResponse {

	c.Log.Infof("%s has entered the run state", c.Name)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-c.ShutdownSignal():
			// ALWAYS watch for shutdown signal
			return rxd.NewResponse(nil, rxd.ExitState)

		case <-timer.C:
			// When 5 seconds has elapsed, log hello, then end the service.
			c.Log.Info("hello")
			return rxd.NewResponse(nil, rxd.StopState)
		}

	}
}

func (s *SimpleService) Init(c *rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.IdleState)
}

func (s *SimpleService) Idle(c *rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.RunState)
}

func (s *SimpleService) Stop(c *rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.ExitState)
}

// SimpleService must meet Service interface or line below errors.
var _ rxd.Service = &SimpleService{}

// Example entrypoint
func main() {
	// We create an instance of our service
	simpleService := NewSimpleService()
	// We create an instance of our ServiceConfig
	svcOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy))
	simpleRxdService := rxd.NewService("SimpleService", simpleService, svcOpts)

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(simpleRxdService)

	// since MyLogger meets the Logging interface we can allow the daemon to use it.
	logger := &MyLogger{}
	daemon.SetCustomLogger(logger)

	err := daemon.Start() // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
