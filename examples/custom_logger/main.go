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

	c.LogInfo("has entered the run state")

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-c.ShutdownSignal():
			// ALWAYS watch for shutdown signal
			return rxd.NewResponse(nil, rxd.ExitState)

		case <-timer.C:
			// When 5 seconds has elapsed, log hello, then end the service.
			c.LogInfo("hello")
			return rxd.NewResponse(nil, rxd.StopState)
		}

	}
}

// Example entrypoint
func main() {
	// We create an instance of our service
	simpleService := NewSimpleService()

	// We create an instance of our ServiceConfig
	svcCtx := rxd.NewServiceContext(
		"SimpleService",
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
	)

	svc := rxd.NewService(svcCtx)

	// We could have used a pure function or you can pass receiver function
	// as long as it meets the interface for stageFunc
	svc.UsingRunFunc(simpleService.Run)

	// can even use an inline function
	svc.UsingStopFunc(func(c *rxd.ServiceContext) rxd.ServiceResponse {
		c.LogInfo("we are stopping now...")
		return rxd.NewResponse(nil, rxd.ExitState)
	})

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(svc)

	// since MyLogger meets the Logging interface we can allow the daemon to use it.
	logger := &MyLogger{}
	daemon.SetLogger(logger)

	err := daemon.Start() // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
