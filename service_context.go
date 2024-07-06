package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
)

type ServiceWatcher interface {
	WatchAllStates(ServiceFilter) (<-chan ServiceStates, context.CancelFunc)
	WatchAnyServices(cfg WatchConfig, services ...string) (<-chan ServiceStates, context.CancelFunc)
	WatchAllServices(cfg WatchConfig, services ...string) (<-chan ServiceStates, context.CancelFunc)
}

type ServiceContext interface {
	context.Context
	ServiceWatcher
	Name() string
	Log(level log.Level, message string)
	With(name string) ServiceContext
}

type serviceContext struct {
	context.Context
	name string
	logC chan<- DaemonLog
	ic   intracom.Intracom[ServiceStates]
}

func newServiceContextWithCancel(parent context.Context, name string, logC chan<- DaemonLog, icStates intracom.Intracom[ServiceStates]) (ServiceContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	return serviceContext{
		Context: ctx,
		name:    name,
		logC:    logC,
		ic:      icStates,
	}, cancel
}

// func (sc serviceContext) Intracom() intracom.Intracom {
// 	return sc.ic
// }

func (sc serviceContext) Name() string {
	return sc.name
}

func (sc serviceContext) With(name string) ServiceContext {
	return serviceContext{
		Context: sc.Context,
		name:    sc.name + "_" + name,
		logC:    sc.logC,
		ic:      sc.ic,
	}
}

func (sc serviceContext) Log(level log.Level, message string) {
	sc.logC <- DaemonLog{
		Level:   level,
		Message: message,
		Name:    sc.name,
	}
}

func (sc serviceContext) Deadline() (deadline time.Time, ok bool) {
	return sc.Context.Deadline()
}

func (sc serviceContext) Done() <-chan struct{} {
	return sc.Context.Done()
}

func (sc serviceContext) Err() error {
	return sc.Context.Err()
}

func (sc serviceContext) Value(key interface{}) interface{} {
	return sc.Context.Value(key)
}

type WatchConfig struct {
	Action         ServiceAction // Entering or Exiting
	TargetState    State         // the state to watch for
	ConsumerSuffix string        // if not spawning child routines from service, leave blank.
}

func (sc serviceContext) WatchAllServices(cfg WatchConfig, services ...string) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer cancel()

		// subscribe to the internal states on behalf of the service context given.
		consumer := internalStatesConsumer(cfg.Action, cfg.TargetState, sc.name)
		sub, unsub := sc.ic.Subscribe(ctx, intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})
		defer unsub()

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
					switch cfg.Action {
					case Entering:
						if val, ok := states[name]; ok && val == cfg.TargetState {
							interestedServices[name] = val
						}

					case Exiting:
						if val, ok := states[name]; ok && val != cfg.TargetState {
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
					}
				}

			}
		}
	}(watchCtx)

	return ch, cancel
}

func (sc serviceContext) WatchAnyServices(cfg WatchConfig, services ...string) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer cancel()

		// subscribe to the internal states on behalf of the service context given.
		consumer := internalStatesConsumer(cfg.Action, cfg.TargetState, sc.name)
		sub, unsub := sc.ic.Subscribe(ctx, intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})
		defer unsub()

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
					switch cfg.Action {
					case Entering:
						if val, ok := states[service]; ok && val == cfg.TargetState {
							interestedServices[service] = val
						}

					case Exiting:
						if val, ok := states[service]; ok && val != cfg.TargetState {
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

func (sc serviceContext) WatchAllStates(filter ServiceFilter) (<-chan ServiceStates, context.CancelFunc) {
	ch := make(chan ServiceStates, 1)
	watchCtx, cancel := context.WithCancel(sc)

	go func(ctx context.Context) {
		defer cancel()

		consumer := internalAllStatesConsumer(sc.name)
		// subscribe to the internal states on behalf of the service context given.
		sub, unsub := sc.ic.Subscribe(ctx, intracom.SubscriberConfig{
			Topic:         internalServiceStates,
			ConsumerGroup: consumer,
			BufferSize:    1,
			BufferPolicy:  intracom.DropOldest,
		})
		defer unsub()

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
