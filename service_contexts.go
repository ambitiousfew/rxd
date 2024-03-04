package rxd

import (
	"context"
	"time"
)

var _ ServiceContext = &serviceContext{}

// ServiceContext is the interface that wraps the context.Context and provides additional functionality.
type ServiceContext interface {
	AnyServicesEnterState(target ServiceState, targetServices ...string) (<-chan StatesResponse, context.CancelFunc)
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

func (c *serviceContext) AnyServicesEnterState(target ServiceState, targetServices ...string) (<-chan StatesResponse, context.CancelFunc) {
	checkC := make(chan StatesResponse, 1)

	ctx, cancel := context.WithCancel(c)

	go func() {
		if c.sw == nil {
			checkC <- StatesResponse{Err: ErrStateWatcherNotSet}
			return
		}
		// subscribe to the internal states on behalf of the service context given.
		consumer := internalEnterStatesConsumer(c.name, target)
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
					if val := states[name]; val == target {
						// build states map of only services we care about.
						interestedServices[name] = val
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
