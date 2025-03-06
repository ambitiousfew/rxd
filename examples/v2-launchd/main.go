package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/sysctl"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logHandler := log.NewHandler(log.WithWriter(os.Stdout))

	serviceLogger := log.NewLogger(log.LevelDebug, logHandler)

	app := application{
		logger: serviceLogger,
	}

	if err := run(ctx, app); err != nil {
		cancel()
		fmt.Println(err)
		serviceLogger.Log(log.LevelError, "Error: %v\n", log.Error("error", err))
		os.Exit(1)
	}
	serviceLogger.Log(log.LevelInfo, "exited normally")
}

type application struct {
	logger log.Logger
}

func run(parent context.Context, app application) (err error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			app.logger.Log(log.LevelError, "Panic: %v\n", log.Any("panic", r))
		}
	}()

	agent := sysctl.NewLaunchdAgent()

	dopts := []rxd.DaemonOption{
		rxd.WithServiceLogger(app.logger),
		rxd.WithSystemAgent(agent),
	}

	d := rxd.NewDaemon("v2-daemon", dopts...)

	err = d.AddService(rxd.Service{
		Name:   "v2-service",
		Runner: &vsService{},
	})
	if err != nil {
		app.logger.Log(log.LevelError, "Error adding service: %v\n", log.Error("error", err))
		return err
	}

	app.logger.Log(log.LevelInfo, "starting daemon")
	// start the daemon
	if err := d.Start(ctx); err != nil {
		app.logger.Log(log.LevelError, "Error starting daemon: %v\n", log.Error("error", err))
		return err
	}

	app.logger.Log(log.LevelInfo, "daemon exited normally")
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
