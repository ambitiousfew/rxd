package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd"
)

// Service Names
const (
	HelloWorldAPI = "HelloWorldAPI"
	PollService   = "PollService"
)

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// create a polling client, poller depends on API service being up.
	pollClient := NewAPIPollingService()
	// create an http api server
	apiServer := NewHelloWorldService()

	l := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Pass N services for daemon to manage and start
	daemon := rxd.NewDaemon(rxd.DaemonConfig{
		Name:    "multi-service-example",
		Signals: []os.Signal{os.Interrupt, syscall.SIGINT, syscall.SIGTERM},
		Opts: []rxd.DaemonOption{
			rxd.UsingLogger(l),
		},
	})

	err := daemon.AddServices(pollClient, apiServer)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	// tell the daemon to Start - this blocks until the underlying
	// services manager stops running, which it wont until all services complete.
	err = daemon.Start(ctx)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	log.Println("daemon has completed")
}
