// For this example we will create two services that will run until they are stopped.
// The first service is a simple HTTP server that listens on port 8000 and responds with a JSON object.
// Response: 200 OK  Body: {"hello": "world"}
//
// This example is meant to show off RxDs ability to have any service subscribe interest to the state of another service.
// The Polling service will Idle until the HelloWorldAPI service enters a RunState.
// Then the Polling service will enter Run and resubscribe interest to the HelloWorldAPI service Exiting its RunState.
// The Polling service can comfortably poll the HelloWorldAPI service for changes in state if the HelloWorldAPI service
// has a sudden change in state causing it to leave its RunState the Polling service will be notified as that happens.
// and can immediately react without Polling an API service that is no longer even running.
package main

import (
	"context"
	"os"
	"time"

	rxd "github.com/ambitiousfew/rxd/v2"
	"github.com/ambitiousfew/rxd/v2/log"
)

// Service Names
const (
	DaemonName           = "multi-service-daemon"
	ServiceHelloWorldAPI = "HelloWorldAPI"
	ServiceAPIPoller     = "PollService"
)

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	daemon := rxd.NewDaemon(DaemonName)

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
