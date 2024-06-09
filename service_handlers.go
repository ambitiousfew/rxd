package rxd

import (
	"context"
	"errors"
	"time"
)

type ServiceHandler interface {
	Handle(ctx context.Context, ds DaemonService, errC chan<- serviceError) error
}

// GetServiceHandler returns the appropriate service handler based on the given policy config
func GetServiceHandler(config RunPolicyConfig) ServiceHandler {
	switch config.Policy {
	case PolicyRunContinous:
		return RunContinuousHandler{
			RestartDelay: config.RestartDelay,
		}
	default:
		return invalidServiceHandler{}
	}
}

// RunContinuousHandler is a service handler that runs the service continuously
// until the context is cancelled or an error occurs.
// RestartDelay is the delay before the service is restarted after it has stopped.
type RunContinuousHandler struct {
	RestartDelay time.Duration
}

func (h RunContinuousHandler) Handle(ctx context.Context, ds DaemonService, errC chan<- serviceError) error {
	state := StateInit
	hasStopped := false
	defaultDelay := 10 * time.Nanosecond
	// NOTE: non-zero initial delay to ensure the first run is fast but stop can reset the timer
	timeout := time.NewTimer(defaultDelay)
	defer timeout.Stop()

	for state != StateExit {
		select {
		case <-ctx.Done():
			state = StateExit
		case <-timeout.C:
			// no signals, continue running, flip stopped back to false
			hasStopped = false
			timeout.Reset(defaultDelay)
		}

		switch state {
		case StateInit:
			if err := ds.Runner.Init(ctx); err != nil {
				errC <- serviceError{
					serviceName: ds.Name,
					err:         err,
					state:       StateInit,
				}
				state = StateStop
			} else {
				state = StateIdle
			}
		case StateIdle:
			if err := ds.Runner.Idle(ctx); err != nil {
				errC <- serviceError{
					serviceName: ds.Name,
					err:         err,
					state:       StateIdle,
				}
				state = StateStop
			} else {
				state = StateRun
			}
		case StateRun:
			if err := ds.Runner.Run(ctx); err != nil {
				errC <- serviceError{
					serviceName: ds.Name,
					err:         err,
					state:       StateIdle,
				}
			}
			state = StateStop

		case StateStop:
			if err := ds.Runner.Stop(ctx); err != nil {
				errC <- serviceError{
					serviceName: ds.Name,
					err:         err,
					state:       StateStop,
				}
			}

			hasStopped = true

			// reset the timer to the configured restart delay before going back to init
			timeout.Reset(h.RestartDelay)
			select {
			case <-ctx.Done():
				state = StateExit
			case <-timeout.C:
				state = StateInit
			}

		}
	}

	// ensuring stop is always run before exiting.
	if !hasStopped {
		err := ds.Runner.Stop(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// invalidServiceHandler is a service handler that always returns an error
type invalidServiceHandler struct{}

func (h invalidServiceHandler) Handle(ctx context.Context, ds DaemonService, errC chan<- serviceError) error {
	return errors.New("invalid service handler")
}
