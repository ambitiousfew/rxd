package main

import (
	"log"
	"os"

	"github.com/ambitiousfew/rxd"
)

// Example entrypoint
func main() {
	// Create Poll Service config with RunPolicy option.
	pollOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunOncePolicy))
	// Pass config to instance of service struct
	pollClient := NewAPIPollingService()
	pollRxdSvc := rxd.NewService("PollService", pollClient, pollOpts)

	apiOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy))
	// Create Hello World service passing config to instance of service struct.
	apiServer := NewHelloWorldService()
	apiSvc := rxd.NewService("HelloWorldAPI", apiServer, apiOpts)

	// We can add polling client as a dependent of API Server so
	// any stage polling client is interested in observing of API Server
	// will be reported down to poll client when API Server reaches that stage.
	apiSvc.AddDependentService(pollRxdSvc, rxd.RunState, rxd.StopState)
	// We are interested in when API Server reaches a RunState and when its reached a StopState
	// NOTE: Make sure you watch for <service context>.ChangeState() in your polling stage that cares.

	// Pass N services for daemon to manage and start
	daemon := rxd.NewDaemon(pollRxdSvc, apiSvc)

	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err := daemon.Start()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
