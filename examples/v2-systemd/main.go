// build +linux
package main

import (
	"context"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

type application struct {
	logger log.Logger
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logger := log.NewLogger(log.LevelDebug, log.NewHandler())

	app := application{
		logger: logger,
	}

	if err := run(ctx, app); err != nil {
		logger.Log(log.LevelError, "error running the daemon", log.Error("error", err))
		os.Exit(1)
	}
	logger.Log(log.LevelInfo, "daemon exited normally")
}

func run(ctx context.Context, app application) error {
	// create a new daemon
	dopts := []rxd.DaemonOption{
		rxd.WithInternalLogging("rxd.log", log.LevelDebug),
		rxd.WithServiceLogger(app.logger),
	}

	d := rxd.NewDaemon("v2-daemon", dopts...)

	err := d.AddService(rxd.Service{
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
