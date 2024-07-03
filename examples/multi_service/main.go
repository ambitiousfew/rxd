package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

// Service Names
const (
	DaemonName    = "multi-service-daemon"
	HelloWorldAPI = "HelloWorldAPI"
	PollService   = "PollService"
)

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Setup a logger for the daemon
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	parentLogger := slog.New(handler)
	// Create Poll Service config with RunPolicy option.
	// Pass config to instance of service struct
	pollClient := NewAPIPollingService(parentLogger.With("service", PollService))
	// Create Hello World service passing config to instance of service struct.
	apiServer := NewHelloWorldService(parentLogger.With("service", HelloWorldAPI))

	apiSvc := rxd.NewService(HelloWorldAPI, apiServer)
	pollSvc := rxd.NewService(PollService, pollClient)
	// We can add polling client as a dependent of API Server so
	// any stage polling client is interested in observing of API Server
	// will be reported down to poll client when API Server reaches that stage.
	// apiSvc.AddDependentService(pollRxdSvc, rxd.RunState, rxd.StopState)
	// We are interested in when API Server reaches a RunState and when its reached a StopState
	// NOTE: Make sure you watch for <service context>.ChangeState() in your polling stage that cares.

	// Pass N services for daemon to manage and start
	daemon := rxd.NewDaemon(DaemonName)

	err := daemon.AddServices(apiSvc, pollSvc)
	if err != nil {
		parentLogger.Error(err.Error())
		os.Exit(1)
	}
	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err = daemon.Start(ctx)
	if err != nil {
		parentLogger.Error(err.Error())
		os.Exit(1)
	}
	parentLogger.Info("daemon has completed")
}
