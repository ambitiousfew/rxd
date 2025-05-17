package main

import (
	"context"
	"encoding/json"
	"errors"
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

var MainConfigDecoderFn = func(p []byte, config *mainServiceConfig) error {
	err := json.Unmarshal(p, config)
	if err != nil {
		return err
	}

	if config.DBPassword == "" {
		return errors.New("db_password is required")
	}

	if config.DBUser == "" {
		return errors.New("db_user is required")
	}

	// Validate the config
	if config.BrokerHost == "" {
		return errors.New("host is required")
	}

	if config.BrokerPort <= 0 {
		return errors.New("port must be greater than 0")
	}
	if config.BrokerPort > 65535 {
		return errors.New("port must be less than 65535")
	}

	if config.BrokerPort < 1024 {
		return errors.New("port must be greater than 1024")
	}

	return nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	handler := log.NewHandler(log.WithWriters(os.Stdout, os.Stderr))

	logger := log.NewLogger(log.LevelDebug, handler).With(log.String("daemon", "v2"))

	mainConfig, err := config.FromFile[mainServiceConfig]("config.json")
	if err != nil {
		logger.Log(log.LevelError, "error loading config", log.Error("error", err))
	}

	app := application{
		logger: logger,
		config: mainConfig,
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

type mainServiceConfig struct {
	DBPassword string `json:"db_password"`
	DBUser     string `json:"db_user"`
	BrokerHost string `json:"broker_host"`
	BrokerPort int    `json:"broker_port"`
	// other fields...
}

func (c mainServiceConfig) Validate(p []byte) error {
	err := json.Unmarshal(p, &c)
	if err != nil {
		return err
	}

	// validate any missing, empty, or invalid fields
	if c.DBPassword == "" {
		return errors.New("db_password is required")
	}
	if c.DBUser == "" {
		return errors.New("db_user is required")
	}
	if c.BrokerHost == "" {
		return errors.New("broker_host is required")
	}
	if c.BrokerPort <= 0 {
		return errors.New("broker_port must be greater than 0")
	}
	if c.BrokerPort > 65535 {
		return errors.New("broker_port must be less than 65535")
	}
	if c.BrokerPort < 1024 {
		return errors.New("broker_port must be greater than 1024")
	}
	return nil
}

type vsService struct {
	timeout *time.Timer
	host    string
	port    int
}

type vsServiceConfig struct {
	BrokerHost string `json:"broker_host"`
	BrokerPort int    `json:"broker_port"`
	// other fields...
}

func (c *vsServiceConfig) Decode(p []byte) error {
	return json.Unmarshal(p, c)
}

func (s *vsService) Load(sctx rxd.ServiceContext, from []byte) error {
	sctx.Log(log.LevelInfo, "service loading")

	// Load the config from the byte array
	var svcConfig vsServiceConfig
	err := config.DecodeFromBytes(from, &svcConfig)
	if err != nil {
		sctx.Log(log.LevelError, "error loading config", log.Error("error", err))
		return nil
	}

	s.host = svcConfig.BrokerHost
	s.port = svcConfig.BrokerPort
	sctx.Log(log.LevelDebug, "loaded config", log.String("host", s.host), log.Int("port", s.port))
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
