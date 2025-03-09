package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
)

type ServiceLogger interface {
	Log(level log.Level, message string, extra ...log.Field)
}

type ServiceWatcher interface {
	WatchAllStates(ServiceFilter) (<-chan ServiceStates, context.CancelFunc)
	WatchAnyServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc)
	WatchAllServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc)
}

type ServiceContext interface {
	context.Context
	ServiceWatcher
	ServiceLogger
	Name() string
	WithFields(fields ...log.Field) ServiceContext
	WithParent(ctx context.Context) ServiceContext
	WithName(name string) (ServiceContext, context.CancelFunc)
}

type serviceContext struct {
	context.Context
	name   string // is the name of the service, can be used for logging/debugging or subscribing.
	fqcn   string // useful for child contexts to have a unique name without having to modify service name when subscribing.
	fields []log.Field
	logC   chan<- DaemonLog
	ic     *intracom.Intracom
}

// newServiceWithCancel produces a new cancellable ServiceContext with the given name and fields.
// func newServiceContextWithCancel(parent context.Context, name string, logC chan<- DaemonLog, icStates intracom.Topic[ServiceStates]) (ServiceContext, context.CancelFunc) {
func NewServiceContextWithCancel(parent context.Context, service string, state DaemonState) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)

	fields := []log.Field{}
	if service != "" {
		fields = append(fields, log.String("service", service))
	}

	return &serviceContext{
		Context: ctx,
		name:    service,
		fqcn:    service,
		fields:  fields,
		logC:    state.logC,
		ic:      state.ic,
	}, cancel
}

// WithParent returns a new cancellable child ServiceContext with the given parent context.
// The new child context will have the same name and fields as the original parent that created it.
// However if the original parent context is cancelled, the child context will not be cancelled.
// The new child will only be cancelled if the new parent context is cancelled.
func (sc *serviceContext) WithParent(parent context.Context) ServiceContext {
	ctx := context.WithoutCancel(parent)

	return &serviceContext{
		Context: ctx,
		name:    sc.name,
		fqcn:    sc.fqcn,
		fields:  sc.fields,
		logC:    sc.logC,
		ic:      sc.ic,
	}
}

// With returns a new child ServiceContext with the given fields appended to the existing fields.
// The new child context will have the same name as the parent.
func (sc *serviceContext) WithFields(fields ...log.Field) ServiceContext {
	return &serviceContext{
		Context: sc.Context,
		name:    sc.name,
		fqcn:    sc.fqcn,
		fields:  append(sc.fields, fields...),
		logC:    sc.logC,
		ic:      sc.ic,
	}
}

func (sc *serviceContext) WithName(name string) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(sc.Context)
	return &serviceContext{
		Context: ctx,
		name:    name,
		fqcn:    sc.fqcn + "_" + name,
		fields:  sc.fields,
		logC:    sc.logC,
		ic:      sc.ic,
	}, cancel
}

func (sc *serviceContext) Name() string {
	return sc.name
}

func (sc *serviceContext) Log(level log.Level, message string, fields ...log.Field) {
	sc.logC <- DaemonLog{
		Level:   level,
		Message: message,
		Fields:  append(fields, sc.fields...),
	}
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

func (sc *serviceContext) WatchAllServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer close(ch)
		// subscribe to the internal states on behalf of the service context given using its "full qualified consumer name" (fqcn).
		consumer := internalStatesConsumer(action, target, sc.fqcn)

		sub, err := intracom.CreateSubscription[ServiceStates](ctx, sc.ic, internalServiceStates, -1, intracom.SubscriberConfig[ServiceStates]{
			ConsumerGroup: consumer,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropOldest[ServiceStates]{},
		})

		if err != nil {
			sc.Log(log.LevelError, "failed to subscribe to internal states: "+err.Error())
			return
		}
		defer intracom.RemoveSubscription[ServiceStates](sc.ic, internalServiceStates, consumer, sub)

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
					case Entered, Entering, Exited, Exiting:
						if val, ok := states[name]; ok && val == target {
							interestedServices[name] = val
						}

					case NotIn:
						if val, ok := states[name]; ok && val != target {
							interestedServices[name] = val
						}
					default:
						// ignore
						continue
					}
				}

				// if we found all those we care about.
				if len(interestedServices) == len(services) {
					select {
					case <-ctx.Done():
						return
					case ch <- interestedServices: // send out the states
						// TODO: should we stop here, or reset and keep collecting the interested services?
					}
				}

			}
		}
	}(watchCtx)

	return ch, cancel
}

func (sc *serviceContext) WatchAnyServices(action ServiceAction, target State, services ...string) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer close(ch)

		// subscribe to the internal states on behalf of the service context given using its "full qualified consumer name" (fqcn).
		consumer := internalStatesConsumer(action, target, sc.fqcn)
		sub, err := intracom.CreateSubscription[ServiceStates](ctx, sc.ic, internalServiceStates, -1, intracom.SubscriberConfig[ServiceStates]{
			ConsumerGroup: consumer,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropOldest[ServiceStates]{},
		})

		if err != nil {
			sc.Log(log.LevelError, "failed to subscribe to internal states: "+err.Error())
			return
		}
		defer intracom.RemoveSubscription[ServiceStates](sc.ic, internalServiceStates, consumer, sub)
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
				for _, service := range services {
					switch action {
					case Entered, Entering, Exited, Exiting:
						if val, ok := states[service]; ok && val == target {
							interestedServices[service] = val
						}
					case NotIn:
						if val, ok := states[service]; ok && val != target {
							interestedServices[service] = val
						}
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

func (sc *serviceContext) WatchAllStates(filter ServiceFilter) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer close(ch)
		// subscribe to the internal states on behalf of the service context given using its "full qualified consumer name" (fqcn).
		consumer := internalAllStatesConsumer(sc.fqcn)
		sub, err := intracom.CreateSubscription[ServiceStates](ctx, sc.ic, internalServiceStates, -1, intracom.SubscriberConfig[ServiceStates]{
			ConsumerGroup: consumer,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropOldest[ServiceStates]{},
		})

		if err != nil {
			sc.Log(log.LevelError, "failed to subscribe to internal states: "+err.Error())
			return
		}
		defer intracom.RemoveSubscription[ServiceStates](sc.ic, internalServiceStates, consumer, sub)

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
