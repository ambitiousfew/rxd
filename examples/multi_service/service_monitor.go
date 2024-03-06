package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

// ServiceStateMonitor
type ServiceMonitor struct {
	log *slog.Logger
}

// NewAPIPollingService just a factory helper function to help create and return a new instance of the service.
func NewServiceMonitor() rxd.Service {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With("service", MonitorService)

	s := &ServiceMonitor{
		log: logger,
	}

	return rxd.Service{
		Conf: rxd.ServiceConfig{
			Name: MonitorService,
		},
		Svc: s,
	}
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *ServiceMonitor) Idle(sc rxd.ServiceContext) rxd.ServiceResponse {
	s.log.Info("service is idling")

	select {
	case <-sc.Done():
		return rxd.NewResponse(nil, rxd.Stop)
	default:
		return rxd.NewResponse(nil, rxd.Run)
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *ServiceMonitor) Run(sc rxd.ServiceContext) rxd.ServiceResponse {

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	s.log.Info("starting service monitor")

	// we dont need to monitor ourselves
	// exclusionFilter := rxd.NewFilterSet(rxd.Exclude, MonitorService)

	statesC, cancel := sc.WatchAllStates(rxd.NoFilter)
	defer cancel()

	serviceStates := make(map[string]string)

	for {
		select {
		case <-sc.Done():
			return rxd.NewResponse(nil, rxd.Stop)
		case <-ticker.C:
			s.log.Info("reporting service states", "states", serviceStates)

		case statesUpdate := <-statesC:
			if statesUpdate.Err != nil {
				s.log.Error(statesUpdate.Err.Error())
				return rxd.NewResponse(nil, rxd.Stop)
			}

			for name, state := range statesUpdate.States {
				serviceStates[name] = state.String()
			}
		}
	}
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *ServiceMonitor) Stop(sc rxd.ServiceContext) rxd.ServiceResponse {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	s.log.Info("service is stopping")
	return rxd.NewResponse(nil, rxd.Exit)
}

func (s *ServiceMonitor) Init(sc rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.Idle)
}

// Ensure we meet the interface or error.
var _ rxd.Servicer = &ServiceMonitor{}
