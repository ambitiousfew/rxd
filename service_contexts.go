package rxd

import (
	"context"
	"time"
)

type ServiceAction int

func (s ServiceAction) String() string {
	switch s {
	case Entering:
		return "entering"
	case Exiting:
		return "exiting"
	default:
		return "unknown"
	}
}

const (
	Entering ServiceAction = iota
	Exiting
)

var _ ServiceContext = &serviceContext{}

type ServiceWatcher interface {
	WatchAllStates(ServiceFilter) (<-chan StatesResponse, context.CancelFunc)
	WatchAnyServices(ServiceAction, ServiceState, ...string) (<-chan StatesResponse, context.CancelFunc)
	WatchAllServices(ServiceAction, ServiceState, ...string) (<-chan StatesResponse, context.CancelFunc)
}

// ServiceContext is the interface that wraps the context.Context and provides additional functionality.
type ServiceContext interface {
	ServiceWatcher
	WithCancel(parent context.Context) (ServiceContext, context.CancelFunc)
	WithTimeout(parent context.Context, timeout time.Duration) (ServiceContext, context.CancelFunc)
	context.Context
}

type StatesResponse struct {
	States ServiceStates
	Err    error
}

// serviceContext extends context.Context and adds additional functionality.
type serviceContext struct {
	name            string        // name of the service for this context
	sw              *stateWatcher // states watcher
	context.Context               // Embedding context.Context
}

func newServiceContext(name string, parent context.Context, sw *stateWatcher) *serviceContext {
	return &serviceContext{name: name, sw: sw, Context: parent}
}

func newServiceContextWithCancel(name string, parent context.Context, sw *stateWatcher) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	return newServiceContext(name, ctx, sw), cancel
}

// SetValue returns a new context with the provided value set.
func (c *serviceContext) SetValue(key, value interface{}) *serviceContext {
	return newServiceContext(c.name, context.WithValue(c.Context, key, value), c.sw)
}

// GetValue retrieves a value from the context.
func (c *serviceContext) GetValue(key interface{}) interface{} {
	return c.Context.Value(key) // Leveraging the embedded context's Value method.
}

// WithCancel returns a new ServiceContext with a cancel function.
func (c *serviceContext) WithCancel(parent context.Context) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	return newServiceContext(c.name, ctx, c.sw), cancel
}

// WithTimeout returns a new ServiceContext with a timeout.
func (c *serviceContext) WithTimeout(parent context.Context, timeout time.Duration) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	return newServiceContext(c.name, ctx, c.sw), cancel
}

// AnyServices returns a channel that will receive the states when ANY of the services given reach the target state based on their action (entering, exiting).
func (c *serviceContext) WatchAnyServices(action ServiceAction, target ServiceState, targetServices ...string) (<-chan StatesResponse, context.CancelFunc) {
	checkC := make(chan StatesResponse, 1)
	ctx, cancel := context.WithCancel(c)

	go func() {
		if c.sw == nil {
			checkC <- StatesResponse{Err: ErrStateWatcherNotSet}
			return
		}

		// subscribe to the internal states on behalf of the service context given.
		consumer := internalStatesConsumer(action, target, c.name)
		subscriberC, err := c.sw.subscribe(ctx, consumer)
		if err != nil {
			checkC <- StatesResponse{Err: err}
			return
		}
		defer c.sw.unsubscribe(ctx, consumer)

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-subscriberC:
				if !open {
					return
				}

				interestedServices := make(ServiceStates, len(targetServices))
				for _, name := range targetServices {
					switch action {
					case Entering:
						if val, ok := states[name]; ok && val == target {
							interestedServices[name] = val
						}

					case Exiting:
						if val, ok := states[name]; ok && val != target {
							interestedServices[name] = val
						}
					}
				}

				// if we found all those we care about.
				if len(interestedServices) > 0 {
					select {
					case <-ctx.Done(): // user cancelled us
						return
					case checkC <- StatesResponse{States: interestedServices, Err: nil}: // send out the states we cared about
					}
				}
			}

		}

	}()

	return checkC, cancel
}

// AllServices returns a channel that will receive the states of the services only after ALL services reach their target state based on their action (entering, exiting).
func (c *serviceContext) WatchAllServices(action ServiceAction, target ServiceState, targetServices ...string) (<-chan StatesResponse, context.CancelFunc) {
	checkC := make(chan StatesResponse, 1)

	ctx, cancel := context.WithCancel(c)

	go func() {
		if c.sw == nil {
			checkC <- StatesResponse{Err: ErrStateWatcherNotSet}
			return
		}
		// subscribe to the internal states on behalf of the service context given.
		consumer := internalStatesConsumer(action, target, c.name)
		subscriberC, err := c.sw.subscribe(ctx, consumer)
		if err != nil {
			checkC <- StatesResponse{Err: err}
			return
		}
		defer c.sw.unsubscribe(ctx, consumer)

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-subscriberC:
				if !open {
					return
				}

				interestedServices := make(ServiceStates, len(targetServices))
				for _, name := range targetServices {
					switch action {
					case Entering:
						if val, ok := states[name]; ok && val == target {
							interestedServices[name] = val
						}

					case Exiting:
						if val, ok := states[name]; ok && val != target {
							interestedServices[name] = val
						}
					}
				}

				// if we found all those we care about.
				if len(interestedServices) == len(targetServices) {
					select {
					case <-ctx.Done():
						return
					case checkC <- StatesResponse{States: interestedServices, Err: nil}: // send out the states
					}
				}

			}
		}
	}()

	return checkC, cancel
}

// AllStates returns a channel that will receive the states of the services given.
// if no service name filters are given, then all states will be returned.
func (c *serviceContext) WatchAllStates(f ServiceFilter) (<-chan StatesResponse, context.CancelFunc) {
	respC := make(chan StatesResponse, 1)

	ctx, cancel := context.WithCancel(c)

	go func() {
		if c.sw == nil {
			respC <- StatesResponse{Err: ErrStateWatcherNotSet}
			return
		}
		// subscribe to the internal states on behalf of the service context given.
		subscriberC, err := c.sw.subscribe(ctx, internalAllStates)
		if err != nil {
			respC <- StatesResponse{Err: err}
			return
		}
		defer c.sw.unsubscribe(ctx, internalAllStates)

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-subscriberC:
				if !open {
					return
				}

				// if no filters are given or mode is set to none, then we just send out all the states we have.
				if len(f.Names) == 0 || f.Mode == None {
					select {
					case <-ctx.Done():
						return
					case respC <- StatesResponse{States: states, Err: nil}:
						// no filtering applied, send out all the states we have.
					}
					continue
				}

				// if we have filters, then we need to filter the states we have.
				filteredInterests := make(ServiceStates, len(f.Names))
				for name, state := range states {
					switch f.Mode {
					case Include:
						// if the FilterSet given contains the service name, then we include it.
						if _, ok := f.Names[name]; ok {
							filteredInterests[name] = state
						}

					case Exclude:
						// if the FilterSet given does not contain the service name, then we include it.
						if _, ok := f.Names[name]; !ok {
							filteredInterests[name] = state
						}
					}
				}

				select {
				case <-ctx.Done():
					return
				case respC <- StatesResponse{States: filteredInterests, Err: nil}: // send out the states
				}
			}
		}
	}()

	return respC, cancel
}
