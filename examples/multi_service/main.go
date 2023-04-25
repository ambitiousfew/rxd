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
	pollSvc := rxd.NewService("PollService", pollClient, pollOpts)

	apiOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy))
	// Create Hello World service passing config to instance of service struct.
	apiServer := NewHelloWorldService()
	apiSvc := rxd.NewService("HelloWorldAPI", apiServer, apiOpts)

	// Pass N services for daemon to manage and start
	daemon := rxd.NewDaemon(pollSvc, apiSvc)

	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err := daemon.Start()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
