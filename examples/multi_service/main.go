package main

import (
	"log"
	"os"

	"github.com/ambitiousfew/rxd"
)

// Example entrypoint
func main() {
	// Create Poll Service config with RunPolicy option.
	pollCfg := rxd.NewServiceConfig(
		rxd.UsingRunPolicy(rxd.RunOncePolicy),
	)
	// Pass config to instance of service struct
	pollSvc := NewAPIPollingService(pollCfg)

	// Create Hello World config with RunPolicy
	//  and Service Notifier so Poll Service can listen
	//  on a channel to be notified of state changes by using
	// service configs exposed: .NotifyStateChange(<state>) method
	apiCfg := rxd.NewServiceConfig(
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
		rxd.UsingServiceNotify(pollSvc),
	)
	// Create Hello World service passing config to instance of service struct.
	apiSvc := NewHelloWorldService(apiCfg)

	// Pass N services for daemon to manage and start
	daemon := rxd.NewDaemon(pollSvc, apiSvc)

	// We can set the log severity we want to observe, LevelInfo is default
	daemon.SetLogSeverity(rxd.LevelAll)

	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err := daemon.Start()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
