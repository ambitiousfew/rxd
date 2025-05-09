package main

import (
	"context"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

// Service Names
const (
	DaemonName           = "multi-service-daemon"
	ServiceHelloWorldAPI = "HelloWorldAPI"
	ServiceAPIPoller     = "PollService"
)

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create Poll Service config with RunPolicy option.
	// Pass config to instance of service struct
	pollClient := NewAPIPollingService()
	// Create Hello World service passing config to instance of service struct.
	apiServer := NewHelloWorldService()

	services := []rxd.Service{
		{
			Name:   ServiceHelloWorldAPI,
			Runner: apiServer,
		},
		{
			Name:   ServiceAPIPoller,
			Runner: pollClient,
		},
	}

	// 1st run
	handler := log.NewHandler(log.WithWriters(os.Stdout, os.Stderr))
	logger := log.NewLogger(log.LevelInfo, handler)

	// 2nd run
	// logger := journald.NewLogger(log.LevelDebug)

	// 3rd run
	// logger := journald.NewLogger(log.LevelInfo, journald.WithSeverityPrefix(true))

	// We can add polling client as a dependent of API Server so
	// any stage polling client is interested in observing of API Server
	// will be reported down to poll client when API Server reaches that stage.
	// apiSvc.AddDependentService(pollRxdSvc, rxd.RunState, rxd.StopState)
	// We are interested in when API Server reaches a RunState and when its reached a StopState
	// NOTE: Make sure you watch for <service context>.ChangeState() in your polling stage that cares.

	// Pass N services for daemon to manage and start

	dopts := []rxd.DaemonOption{
		rxd.WithRPC(rxd.RPCConfig{
			Addr: "localhost",
			Port: 1337,
		}),
	}
	daemon := rxd.NewDaemon(DaemonName, dopts...)

	err := daemon.AddServices(services...)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}
	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err = daemon.Start(ctx)
	if err != nil {
		logger.Log(log.LevelError, err.Error())
		os.Exit(1)
	}
	logger.Log(log.LevelInfo, "successfully stopped daemon")
}
