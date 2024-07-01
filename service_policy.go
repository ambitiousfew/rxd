package rxd

import (
	"context"
	"errors"
)

const (
	// PolicyContinue will continue to run the service even if an error occurs.
	PolicyContinue RunPolicy = iota
	// PolicyStop will stop the service if an error occurs.
	PolicyStop
	// PolicyRestart will restart the service if an error occurs.
	PolicyRestart
)

type RunPolicy uint8

func (p RunPolicy) String() string {
	switch p {
	case PolicyContinue:
		return "Continue"
	case PolicyStop:
		return "Stop"
	case PolicyRestart:
		return "Restart"
	default:
		return "Unknown"
	}
}

func (p RunPolicy) Handle(ctx context.Context, service DaemonService, errC chan<- error) {
	switch p {
	case PolicyContinue:
		handlePolicyContinue(ctx, service, errC)
	default:
		errC <- errors.New("unknown policy")
		return
	}
}

func handlePolicyContinue(ctx context.Context, service DaemonService, errC chan<- error) {
	serviceCtx := NewServiceContext(ctx, service.Name)

	var hasStopped bool
	var state State

	for state != Exit {
		select {
		case <-serviceCtx.Done():
			state = Exit

		default:
			switch state {
			case Init:
				state = Idle
				err := service.Runner.Init(serviceCtx)
				if err != nil {
					errC <- err
					return
				}
			case Idle:
				state = Run
				err := service.Runner.Idle(serviceCtx)
				if err != nil {
					errC <- err
					return
				}
			case Run:
				state = Stop
				err := service.Runner.Run(serviceCtx)
				if err != nil {
					errC <- err
					return
				}
			case Stop:
				state = Init
				err := service.Runner.Stop(serviceCtx)
				if err != nil {
					errC <- err
					return
				}
				hasStopped = true
			}
		}
	}

	if !hasStopped {
		err := service.Runner.Stop(serviceCtx)
		if err != nil {
			errC <- err
		}
	}

}
