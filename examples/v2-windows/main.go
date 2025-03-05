package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/sysctl"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx); err != nil {
		// log.Fatalf("Error: %v\n", err)
		fmt.Println("Error: ", err)
	}
	fmt.Println("Exiting...")
}

func run(ctx context.Context) error {
	debugFilePath := filepath.Join("X:", "code", "dev", "go", "rxd", "examples", "v2-windows", "rxd-debug.log")
	debugFile, err := os.Create(debugFilePath)
	if err != nil {
		return err
	}
	defer debugFile.Close()
	defer func() {
		if r := recover(); r != nil {
			debugFile.WriteString(fmt.Sprintf("Panic: %v\n", r))
		}
	}()

	agent, err := sysctl.NewWindowsSCMAgent("v2-daemon")
	if err != nil {
		debugFile.WriteString(fmt.Sprintf("Error creating agent: %v\n", err))
		return err
	}

	// create a new daemon
	dopts := []rxd.DaemonOption{
		rxd.WithInternalLogging("rxd.log", log.LevelDebug),
		rxd.WithDaemonAgent(agent),
	}

	d := rxd.NewDaemon("v2-daemon", dopts...)

	err = d.AddService(rxd.Service{
		Name:   "v2-service",
		Runner: &vsService{},
	})
	if err != nil {
		debugFile.WriteString(fmt.Sprintf("Error adding service: %v\n", err))
		return err
	}

	// start the daemon
	if err := d.Start(ctx); err != nil {
		debugFile.WriteString(fmt.Sprintf("Error starting daemon: %v\n", err))
		return err
	}
	debugFile.WriteString("Daemon exited normally\n")
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
