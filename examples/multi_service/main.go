package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"golang.org/x/exp/slog"
)

// Service Names
const (
	HelloWorldAPI = "HelloWorldAPI"
	PollService   = "PollService"
)

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create Poll Service config with RunPolicy option.
	pollOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunOncePolicy))
	// Pass config to instance of service struct
	pollClient := NewAPIPollingService()
	pollRxdSvc := rxd.NewService(PollService, pollClient, pollOpts)

	apiOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy))
	// Create Hello World service passing config to instance of service struct.
	apiServer := NewHelloWorldService()
	apiSvc := rxd.NewService(HelloWorldAPI, apiServer, apiOpts)

	// We can add polling client as a dependent of API Server so
	// any stage polling client is interested in observing of API Server
	// will be reported down to poll client when API Server reaches that stage.
	// apiSvc.AddDependentService(pollRxdSvc, rxd.RunState, rxd.StopState)
	// We are interested in when API Server reaches a RunState and when its reached a StopState
	// NOTE: Make sure you watch for <service context>.ChangeState() in your polling stage that cares.

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	// Pass N services for daemon to manage and start
	daemon := rxd.NewDaemon(pollRxdSvc, apiSvc)
	daemon.SetLogHandler(handler)
	// daemon.SetIntracom(ic) // optional step to display DEBUG logs from intracom

	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err := daemon.Start(ctx)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	logger.Info("daemon has completed")
}
