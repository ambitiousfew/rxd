package main

import (
	"context"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/config"
	"github.com/ambitiousfew/rxd/log"
)

type application struct {
	logger log.Logger
	config config.ReadLoader
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	handler := log.NewHandler(log.WithWriters(os.Stdout, os.Stderr))

	logger := log.NewLogger(log.LevelDebug, handler).With(log.String("daemon", "v2"))

	config, err := config.FromFile("config.json")
	if err != nil {
		logger.Log(log.LevelError, "error loading config", log.Error("error", err))
	}

	app := application{
		logger: logger,
		config: config,
	}

	if err := run(ctx, app); err != nil {
		logger.Log(log.LevelError, "error running the daemon", log.Error("error", err))
		os.Exit(1)
	}

	logger.Log(log.LevelInfo, "exited normally")
}

func run(ctx context.Context, app application) error {
	// create a new daemon
	dopts := []rxd.DaemonOption{
		rxd.WithInternalLogger(app.logger),
		rxd.WithServiceLogger(app.logger),
		rxd.WithConfigLoader(app.config),
		rxd.WithRPC(rxd.RPCConfig{
			Addr: "127.0.0.1",
			Port: 8080,
		}),
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
	host    string
	port    int
}

func (s *vsService) Load(sctx rxd.ServiceContext, fields map[string]any) error {
	sctx.Log(log.LevelInfo, "service loading")

	if host, ok := fields["service_host"].(string); ok {
		s.host = host
	} else {
		sctx.Log(log.LevelError, "host not found in config")
	}

	if port, ok := fields["service_port"].(float64); ok {
		s.port = int(port)
	} else {
		sctx.Log(log.LevelError, "port not found in config")
	}

	return nil
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
	sctx.Log(log.LevelDebug, "current config", log.String("host", s.host), log.Int("port", s.port))
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
