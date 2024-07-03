package rxd

// ServiceHandler interface defines the methods that a service handler must implement
type ServiceHandler interface {
	Handle(ctx ServiceContext, service Service, errC chan<- error)
}

// ServiceHandlerContinous is a service handler that runs the service continuously
// either until an OS signal or the daemon Start context is cancelled.
// This is the default handler for a service unless specified otherwise by overriding with
// the UsingHandler service option.
type ServiceHandlerContinous struct{}

func (h ServiceHandlerContinous) Handle(ctx ServiceContext, service Service, errC chan<- error) {
	var hasStopped bool
	var state State

	for state != Exit {
		select {
		case <-ctx.Done():
			state = Exit
		default:
			switch state {
			case Init:
				state = Idle
				err := service.Runner.Init(ctx)
				if err != nil {
					errC <- err
					state = Exit // or Stop with timeout?
				}
			case Idle:
				state = Run
				err := service.Runner.Idle(ctx)
				if err != nil {
					errC <- err
					state = Stop
				}
			case Run:
				state = Stop
				err := service.Runner.Run(ctx)
				if err != nil {
					errC <- err
				}
			case Stop:
				state = Init
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

type ServiceHandlerRunOnce struct{}

func (h ServiceHandlerRunOnce) Handle(ctx ServiceContext, service Service, errC chan<- error) {
	var state State

	for state != Exit {
		select {
		case <-ctx.Done():
			state = Exit
		default:
			switch state {
			case Init:
				state = Run
				err := service.Runner.Init(ctx)
				if err != nil {
					errC <- err
					state = Exit
				}
			case Run:
				state = Exit
				err := service.Runner.Run(ctx)
				if err != nil {
					errC <- err
				}
			}
		}
	}

	err := service.Runner.Stop(ctx)
	if err != nil {
		errC <- err
	}
}
