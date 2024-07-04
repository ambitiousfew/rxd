package rxd

// ServiceHandler interface defines the methods that a service handler must implement
type ServiceHandler interface {
	Handle(ctx ServiceContext, service service, errC chan<- error)
}

// DefaultHandler is a service handler that runs the service continuously
// either until an OS signal or the daemon Start context is cancelled.
// This is the default handler for a service unless specified otherwise by overriding with
// the UsingHandler service option.
type DefaultHandler struct{}

// Handle runs the service continuously until the context is cancelled.
// service contains the service runner that will be executed.
// errC is a channel that is used to report errors that occur during the service execution.
// which is then handled by the daemon.
func (h DefaultHandler) Handle(ctx ServiceContext, service service, errC chan<- error) {
	var hasStopped bool
	var state State

	for state != StateExit {
		select {
		case <-ctx.Done():
			state = StateExit
		default:
			switch state {
			case StateInit:
				state = StateIdle
				err := service.Runner.Init(ctx)
				if err != nil {
					errC <- err
					state = StateExit // or Stop with timeout?
				}
			case StateIdle:
				state = StateRun
				err := service.Runner.Idle(ctx)
				if err != nil {
					errC <- err
					state = StateStop
				}
			case StateRun:
				state = StateStop
				err := service.Runner.Run(ctx)
				if err != nil {
					errC <- err
				}
			case StateStop:
				state = StateInit
				err := service.Runner.Stop(ctx)
				if err != nil {
					errC <- err
				}

				// TODO: Add a timeout to the stop state to prevent
				// the service from cycling too quickly between retries.
				hasStopped = true
			}
		}
	}

	if !hasStopped {
		err := service.Runner.Stop(ctx)
		if err != nil {
			errC <- err
		}
	}

}
