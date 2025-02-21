package main

import (
	"context"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	log.Println("exiting...")
}

func run(ctx context.Context) error {
	// create a new daemon
	dopts := []rxd.DaemonOption{
		rxd.WithSignals(os.Interrupt, syscall.SIGTERM),
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

type vsService struct{}

func (s *vsService) Init(sctx rxd.ServiceContext) error {
	log.Println("service init")
	select {
	case <-sctx.Done():
		return nil
	default:

		return nil
	}
}

func (s *vsService) Idle(sctx rxd.ServiceContext) error {
	log.Println("service idle")
	select {
	case <-sctx.Done():
		return nil
	default:

		return nil
	}
}

func (s *vsService) Run(sctx rxd.ServiceContext) error {
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()

	log.Println("service run")
	select {
	case <-sctx.Done():
		return nil
	case <-timeout.C:
		log.Println("service run complete")
		return nil
	}
}

func (s *vsService) Stop(sctx rxd.ServiceContext) error {
	log.Println("service stop")

	select {
	case <-sctx.Done():
		return nil
	default:

		return nil
	}
}
