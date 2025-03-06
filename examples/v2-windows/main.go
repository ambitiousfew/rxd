package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logPath := filepath.Join("v2-windows.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer logFile.Close()

	serviceLogger := log.NewLogger(log.LevelDebug, log.NewHandler(log.WithWriter(logFile)))

	app := application{
		logger: serviceLogger,
	}

	if err := run(ctx, app); err != nil {
		cancel()
		fmt.Println(err)
		serviceLogger.Log(log.LevelError, "Error: %v\n", log.Error("error", err))
		logFile.Close()
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

	// If you want to build a binary that you can run as a service using Windows SCM, such as:
	// sc.exe create <your-service-name> binPath= "C:\path\to\your\binary.exe"
	// You will need to create a new agent passing in the service name.
	//
	// NOTE: The <your-service-name> must be unique and sc.exe <your-service-name>
	// must match the same name as you pass to the NewWindowsSCMAgent function.

	// agent, err = sysctl.NewWindowsSCMAgent("<your-service-name>")
	// if err != nil {
	// 	app.logger.Log(log.LevelError, "Error creating agent: %v\n", log.Error("error", err))
	// 	return err
	// }

	// The default agent is being used below (by default for all OSes)
	// runs similar to a normal process that would block your terminal.
	// It does not interact with the service control manager but it does
	// respond to Interrupt / SIGINT / SIGTERM from the OS.
	dopts := []rxd.DaemonOption{
		rxd.WithServiceLogger(app.logger),
		// rxd.WithSystemAgent(agent),
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
