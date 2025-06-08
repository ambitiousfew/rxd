package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
)

// ServiceLogger is an interface for logging messages within a service context.
// It allows logging at different levels with additional fields for context.
// This interface is used by the ServiceContext to log messages related to service operations.
// It provides a way to log messages with a specific level and additional fields for context.
type ServiceLogger interface {
	Log(level log.Level, message string, extra ...log.Field)
}

// ServiceWatcher is an interface for watching service states.
type ServiceWatcher interface {
	WatchAllStates(ServiceFilter) (<-chan ServiceStates, context.CancelFunc)
	WatchAnyServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc)
	WatchAllServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc)
}

// ServiceContext is an interface that combines context.Context, ServiceWatcher, and ServiceLogger.
// It provides a way to manage service-specific contexts with logging and state watching capabilities.
// It allows for creating child contexts with specific names and fields, and provides methods to log messages
type ServiceContext interface {
	context.Context
	ServiceWatcher
	ServiceLogger
	WithFields(fields ...log.Field) ServiceContext
	WithParent(ctx context.Context) (ServiceContext, context.CancelFunc)
	WithName(name string) (ServiceContext, context.CancelFunc)
	IntracomRegistry() *intracom.Intracom
}

type serviceContext struct {
	context.Context
	name   string             // is the name of the service, can be used for logging/debugging or subscribing.
	fqcn   string             // useful for child contexts to have a unique name without having to modify service name when subscribing.
	fields []log.Field        // fields is a slice of log fields that are used for logging and debugging.
	logger log.Logger         // logger is the logger used for logging messages within the service context.
	ic     *intracom.Intracom // ic is the shared intracom registry used between all the services.
}

func newServiceContextWithCancel(parent context.Context, name string, logger log.Logger, ic *intracom.Intracom) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	fields := []log.Field{}
	if name != "" {
		fields = append(fields, log.String("service", name))
	}

	return &serviceContext{
		Context: ctx,
		name:    name,
		fqcn:    name,
		fields:  fields,
		logger:  logger,
		ic:      ic,
	}, cancel
}

// WithParent replaces the original context with the provided parent context.
// It returns the original ServiceContext with the new context and a cancel function.
// This allows for detaching the service context from its parent and allowing the caller to
// cancel the context at an earlier time if needed.
func (sc *serviceContext) WithParent(parent context.Context) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	newCtx := *sc
	newCtx.Context = ctx
	return &newCtx, cancel
}

// WithFields allows the caller to add additional fields to the service context
// so when logging messages, these fields are always attached to all log messages.
func (sc *serviceContext) WithFields(fields ...log.Field) ServiceContext {
	newCtx := *sc
	newCtx.fields = append(sc.fields, fields...)
	return &newCtx
}

// WithName allows the caller to create a new CHILD service context with a specific name.
// This is useful for creating child contexts to be used for services launches additional
// worker goroutines where the parent service is managing its own lifecycles of those workers.
// The returned ServiceContext is composed from the original parent context, therefore it has
// if the parent is cancelled, the child context will also be cancelled when the daemon is shutting down.
func (sc *serviceContext) WithName(name string) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(sc.Context)
	newCtx := *sc
	newCtx.Context = ctx
	newCtx.name = name
	newCtx.fqcn = sc.fqcn + "_" + name
	return &newCtx, cancel
}

// Log logs a message with the specified level, message, and any additional fields.
func (sc *serviceContext) Log(level log.Level, message string, fields ...log.Field) {
	allFields := make([]log.Field, 0, len(fields)+len(sc.fields))
	allFields = append(sc.fields, fields...)
	sc.logger.Log(level, message, allFields...)
}

func (sc *serviceContext) Deadline() (deadline time.Time, ok bool) {
	return sc.Context.Deadline()
}

func (sc *serviceContext) Done() <-chan struct{} {
	return sc.Context.Done()
}

func (sc *serviceContext) Err() error {
	return sc.Context.Err()
}

func (sc *serviceContext) Value(key interface{}) interface{} {
	return sc.Context.Value(key)
}

// IntracomRegistry returns the embedded intracom registry used by the daemon for all services.
// The caller can use this to register new topics, create subscriptions, and publish messages
// between services.
// NOTE: Avoid using topics with an _rxd prefix as these are reserved for internal use by the daemon.
// overlapping this topic name may cause unexpected behavior with internal states tracking by the daemon.
func (sc *serviceContext) IntracomRegistry() *intracom.Intracom {
	return sc.ic
}

// WatchAllServices allows the caller to specify a list of services by unique name and their target state and action.
// It returns a channel that will only signal when ALL the services given match the action and target state.
// Only once all the services reach the desired state, the channel will send out the states of those services as a signal.
// This is useful for scenarios where you want to wait for multiple services to reach a specific state before proceeding.
//
// NOTE: The 2nd return value is a cancel function that can be used to stop watching the services.
// The channel will stay open until this cancel function is called or the parent context is done.
// The caller should always call the cancel function to avoid leaking resources.
func (sc *serviceContext) WatchAllServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer close(ch)
		// subscribe to the internal states on behalf of the service context given using its "full qualified consumer name" (fqcn).
		consumer := internalStatesConsumer(action, target, sc.fqcn)

		sub, err := intracom.CreateSubscription(ctx, sc.ic, internalServiceStates, -1, intracom.SubscriberConfig[ServiceStates]{
			ConsumerGroup: consumer,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropOldest[ServiceStates]{},
		})

		if err != nil {
			sc.Log(log.LevelError, "failed to subscribe to internal states: "+err.Error())
			return
		}
		defer intracom.RemoveSubscription(sc.ic, internalServiceStates, consumer, sub)

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-sub:
				if !open {
					return
				}

				interestedServices := make(ServiceStates, len(services))
				for _, name := range services {
					switch action {
					case Entered, Entering:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}
						// if the current state matches the target state and the transition is entering, we are interested in it.
						if current.State == target && current.Transition == TransitionEntering {
							interestedServices[name] = current
						}

					case Exited, Exiting:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}

						// if the current state matches the target state and the transition is exited, we are interested in it.
						if current.State == target && current.Transition == TransitionExited {
							interestedServices[name] = current
						}

					case Changed, Changing:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}

						// if the current state matches the target state, we are interested in it no matter the transition.
						if current.State == target {
							interestedServices[name] = current
						}

					case NotIn:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}
						// if the current state does not match the target state, we are interested in it.
						if current.State != target {
							interestedServices[name] = current
						}
					default: // ignore
					}
				}

				// if we found all those we care about.
				if len(interestedServices) == len(services) {
					select {
					case <-ctx.Done():
						sc.logger.Log(log.LevelDebug, "context done, stopping watch for interested services")
						return
					case ch <- interestedServices: // send out the states to caller
						// NOTE: we keep the channel open because we dont assume the caller is done after the first signal.
						// if they wish to stop watching they should cancel using the provided cancel function returned by this method.
					}
				}

			}
		}
	}(watchCtx)

	return ch, cancel
}

// WatchAnyServices allows the caller to specify a list of services by unique name and their target state and action.
// It returns a channel that will signal when ANY of the services given match the action and target state.
// The moment a single service reaches the desired state, the channel will send out the states of those services as a signal.
// This is useful for scenarios where you want to be notified when any group of services reaches a specific state.
//
// NOTE: The 2nd return value is a cancel function that can be used to stop watching the services.
// The channel will stay open until this cancel function is called or the parent context is done.
// The caller should always call the cancel function to avoid leaking resources.
// This is useful for scenarios where you want to be notified when any of the services reaches a specific state.
func (sc *serviceContext) WatchAnyServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer close(ch)

		// subscribe to the internal states on behalf of the service context given using its "full qualified consumer name" (fqcn).
		consumer := internalStatesConsumer(action, target, sc.fqcn)
		sub, err := intracom.CreateSubscription(ctx, sc.ic, internalServiceStates, -1, intracom.SubscriberConfig[ServiceStates]{
			ConsumerGroup: consumer,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropOldest[ServiceStates]{},
		})

		if err != nil {
			sc.Log(log.LevelError, "failed to subscribe to internal states: "+err.Error())
			return
		}
		defer intracom.RemoveSubscription(sc.ic, internalServiceStates, consumer, sub)
		// defer sc.icStates.Unsubscribe(consumer, sub)

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-sub:
				if !open {
					return
				}

				interestedServices := make(ServiceStates, len(services))
				for _, name := range services {
					switch action {
					case Entered, Entering:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}
						// if the current state matches the target state and the transition is entering, we are interested in it.
						if current.State == target && current.Transition == TransitionEntering {
							interestedServices[name] = current
						}

					case Exited, Exiting:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}

						// if the current state matches the target state and the transition is exited, we are interested in it.
						if current.State == target && current.Transition == TransitionExited {
							interestedServices[name] = current
						}

					case Changed, Changing:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}

						// if the current state matches the target state, we are interested in it no matter the transition.
						if current.State == target {
							interestedServices[name] = current
						}

					case NotIn:
						current, ok := states[name]
						if !ok {
							sc.logger.Log(log.LevelWarning, "service not found in states", log.String("service", name), log.String("consumer", consumer))
							continue // skip if the service is not found in the states
						}
						// if the current state does not match the target state, we are interested in it.
						if current.State != target {
							interestedServices[name] = current
						}

					default: // ignore
					}
				}

				// if we found all those we care about.
				if len(interestedServices) > 0 {
					select {
					case <-ctx.Done(): // user cancelled us
						return
					case ch <- interestedServices: // send out the states we cared about
					}
				}
			}

		}

	}(watchCtx)

	return ch, cancel
}

// WatchAllStates allows the caller to watch all service states and filter them based on the provided filter.
// It returns a channel that will signal every time there is a change in the states of the services.
// The channel will send out the current states of all services that match the filter criteria.
// If no filter is provided, it will send out all the states of all services for every single change.
// The filter can be used to include or exclude specific services based on their names.
// This is useful for scenarios where you want to monitor the states of all services from another reporting-like service.
//
// NOTE: The 2nd return value is a cancel function that can be used to stop watching the services.
// The channel will stay open until this cancel function is called or the parent context is done.
// The caller should always call the cancel function to avoid leaking resources.
// The channel will send out the states of all services that match the filter criteria.
func (sc *serviceContext) WatchAllStates(filter ServiceFilter) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer close(ch)
		// subscribe to the internal states on behalf of the service context given using its "full qualified consumer name" (fqcn).
		consumer := internalAllStatesConsumer(sc.fqcn)
		sub, err := intracom.CreateSubscription(ctx, sc.ic, internalServiceStates, -1, intracom.SubscriberConfig[ServiceStates]{
			ConsumerGroup: consumer,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropOldest[ServiceStates]{},
		})

		if err != nil {
			sc.Log(log.LevelError, "failed to subscribe to internal states: "+err.Error())
			return
		}
		defer intracom.RemoveSubscription(sc.ic, internalServiceStates, consumer, sub)

		for {
			select {
			case <-ctx.Done():
				return

			case states, open := <-sub:
				if !open {
					return
				}

				// if no filters are given or mode is set to none, then we just send out all the states we have.
				if len(filter.Names) == 0 || filter.Mode == None {
					select {
					case <-ctx.Done():
						return
					case ch <- states:
						// no filtering applied, send out all the states we have.
					}
					continue
				}

				// if we have filters, then we need to filter the states we have.
				filteredInterests := make(ServiceStates, len(filter.Names))
				for name, state := range states {
					switch filter.Mode {
					case Include:
						// if the FilterSet given contains the service name, then we include it.
						if _, ok := filter.Names[name]; ok {
							filteredInterests[name] = state
						}

					case Exclude:
						// if the FilterSet given does not contain the service name, then we include it.
						if _, ok := filter.Names[name]; !ok {
							filteredInterests[name] = state
						}
					}
				}

				select {
				case <-ctx.Done():
					return
				case ch <- filteredInterests: // send out the states
				}
			}
		}
	}(watchCtx)

	return ch, cancel
}
