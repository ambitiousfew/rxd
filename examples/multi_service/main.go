package main

import (
	"log"
	"os"

	"github.com/ambitiousfew/rxd"
)

// Example entrypoint
func main() {
	// Create Poll Service config with RunPolicy option.
	pollCfg := rxd.NewServiceContext(
		"PollService",
		rxd.UsingRunPolicy(rxd.RunOncePolicy),
	)
	// Pass config to instance of service struct
	pollClient := NewAPIPollingService()

	pollSvc := rxd.NewService(pollCfg)
	pollSvc.UsingIdleFunc(pollClient.Idle)
	pollSvc.UsingRunFunc(pollClient.Run)

	apiCfg := rxd.NewServiceContext(
		"HelloWorldAPI",
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
		rxd.UsingServiceNotify(pollSvc),
	)
	// Create Hello World service passing config to instance of service struct.
	apiServer := NewHelloWorldService()
	apiSvc := rxd.NewService(apiCfg)
	apiSvc.UsingInitFunc(apiServer.Idle)
	apiSvc.UsingRunFunc(apiServer.Run)
	apiSvc.UsingStopFunc(apiServer.Stop)

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
