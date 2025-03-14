// build +linux
package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/config"
	"github.com/ambitiousfew/rxd/log"
)

type application struct {
	logger     log.Logger
	configFile string
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logger := log.NewLogger(log.LevelDebug, log.NewHandler()).With(log.String("daemon", "v2-systemd"))

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

	configReadLoader := config.FromJSONFile("config.json")

	// create a new daemon
	dopts := []rxd.DaemonOption{
		rxd.WithInternalLogging("rxd.log", log.LevelDebug),
		rxd.WithServiceLogger(app.logger),
		rxd.WithConfigLoader(configReadLoader),
	}

	d := rxd.NewDaemon("v2-daemon", dopts...)

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	vs := &vsService{
		timeout: timer,
		myField: "not-spicy",
		mu:      sync.RWMutex{},
	}

	err := d.AddService(rxd.Service{
		Name:   "v2-service",
		Runner: vs,
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

	myField string
	mu      sync.RWMutex
}

// func (s *vsService) Init(sctx rxd.ServiceContext) error {
// 	sctx.Log(log.LevelInfo, "service init")
// 	if s.timeout == nil {
// 		s.timeout = time.NewTimer(3 * time.Second)
// 	} else {
// 		s.timeout.Reset(3 * time.Second)
// 	}

// 	select {
// 	case <-sctx.Done():
// 		return nil
// 	case <-s.timeout.C:
// 		sctx.Log(log.LevelDebug, "service init complete")
// 		return nil
// 	}
// }

// func (s *vsService) Idle(sctx rxd.ServiceContext) error {
// 	s.timeout.Reset(1 * time.Second)
// 	sctx.Log(log.LevelInfo, "service idle")
// 	select {
// 	case <-sctx.Done():
// 		return nil
// 	case <-s.timeout.C:
// 		s.mu.RLock()
// 		sctx.Log(log.LevelDebug, "service idle complete: "+s.myField)
// 		s.mu.RUnlock()
// 		return nil
// 	}
// }

func (s *vsService) Reload(sctx rxd.ServiceContext, fields map[string]any) error {
	s.timeout.Reset(3 * time.Second)

	sctx.Log(log.LevelNotice, "service is being reloaded")
	for k, v := range fields {
		sctx.Log(log.LevelDebug, "field: "+k, log.Any("value", v))
	}

	select {
	case <-sctx.Done():
	case <-s.timeout.C:
		sctx.Log(log.LevelNotice, "service is done reloading")
	}

	return nil
}

func (s *vsService) Run(sctx rxd.ServiceContext) error {
	s.timeout.Reset(30 * time.Second)
	sctx.Log(log.LevelInfo, "service run")
	select {
	case <-sctx.Done():
		sctx.Log(log.LevelDebug, "service run receiving context done, exiting")
		return nil
	case <-s.timeout.C:
		s.mu.RLock()
		sctx.Log(log.LevelDebug, "service run complete: "+s.myField)
		s.mu.RUnlock()
		sctx.Log(log.LevelError, "logging an error for funsies")
		return nil
	}
}

// func (s *vsService) Stop(sctx rxd.ServiceContext) error {
// 	s.timeout.Reset(1 * time.Second)
// 	sctx.Log(log.LevelInfo, "service stop")

// 	select {
// 	case <-sctx.Done():
// 		return nil
// 	case <-s.timeout.C:
// 		sctx.Log(log.LevelDebug, "service stop complete")
// 		return nil
// 	}

// }
