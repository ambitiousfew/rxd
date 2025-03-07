//go:build windows

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/sysctl"
)

// This name must match the name used to
// create the service with the `sc create` command.
const SCMDaemonName = "v2-daemon"

type application struct {
	daemonName string
	logger     log.Logger
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewLogger(log.LevelDebug, log.NewHandler())

	app := application{
		daemonName: SCMDaemonName,
		logger:     logger,
	}

	if err := run(ctx, app); err != nil {
		logger.Log(log.LevelError, "error running the daemon", log.Error("error", err))
	}
	logger.Log(log.LevelInfo, "daemon exited normally")
}

func run(ctx context.Context, app application) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	agent, err := sysctl.NewWindowsSCMAgent(app.daemonName)
	// create a new daemon
	dopts := []rxd.DaemonOption{
		rxd.WithInternalLogging("rxd.log", log.LevelDebug),
		rxd.WithServiceLogger(app.logger),
		rxd.WithDaemonAgent(agent),
	}

	d := rxd.NewDaemon(app.daemonName, dopts...)

	err = d.AddService(rxd.Service{
		Name:   "v2-service",
		Runner: &vsService{},
	})
	if err != nil {
		return err
	}

	// start the daemon
	if err := d.Start(ctx); err != nil {
		return err
	}
	return nil
}

type vsService struct {
	timeout *time.Timer
}

func (s *vsService) Init(sctx rxd.ServiceContext) error {
	sctx.Log(log.LevelInfo, "service init")
	if s.timeout == nil {
		s.timeout = time.NewTimer(3 * time.Second)
	} else {
		s.timeout.Reset(3 * time.Second)
	}

	select {
	case <-sctx.Done():
		return nil
	case <-s.timeout.C:
		sctx.Log(log.LevelDebug, "service init complete")
		return nil
	}
}

func (s *vsService) Idle(sctx rxd.ServiceContext) error {
	s.timeout.Reset(1 * time.Second)
	sctx.Log(log.LevelInfo, "service idle")
	select {
	case <-sctx.Done():
		return nil
	case <-s.timeout.C:
		sctx.Log(log.LevelDebug, "service idle complete")
		return nil
	}
}

func (s *vsService) Run(sctx rxd.ServiceContext) error {
	s.timeout.Reset(1 * time.Second)
	sctx.Log(log.LevelInfo, "service run")
	select {
	case <-sctx.Done():
		return nil
	case <-s.timeout.C:
		sctx.Log(log.LevelDebug, "service run complete")
		sctx.Log(log.LevelError, "logging an error for funsies")
		return nil
	}
}

func (s *vsService) Stop(sctx rxd.ServiceContext) error {
	s.timeout.Reset(1 * time.Second)
	sctx.Log(log.LevelInfo, "service stop")

	select {
	case <-sctx.Done():
		return nil
	case <-s.timeout.C:
		sctx.Log(log.LevelDebug, "service stop complete")
		return nil
	}

}
