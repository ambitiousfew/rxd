package main

import (
	"log"
	"os"

	"github.com/ambitiousfew/rxd"
)

// Example entrypoint
func main() {
	// We create an instance of our ServiceConfig
	apiCfg := rxd.NewServiceConfig(
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
	)
	// We create an instance of our service
	apiSvc := NewHelloWorldService(apiCfg)

	pollCfg := rxd.NewServiceConfig(
		rxd.UsingRunPolicy(rxd.RunOncePolicy),
	)
	pollSvc := NewAPIPollingService(pollCfg)

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(apiSvc, pollSvc)

	// We can set the log severity we want to observe, LevelInfo is default
	daemon.SetLogSeverity(rxd.LevelInfo)

	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err := daemon.Start()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
