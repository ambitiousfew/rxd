package rxd

import (
	"context"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
)

type Daemon interface {
	AddServices(services ...Service) error
	AddService(service Service) error
	Start(ctx context.Context) error
}

type daemon struct {
	name            string                           // name of the daemon will be used in logging
	signals         []os.Signal                      // OS signals you want your daemon to listen for
	services        map[string]DaemonService         // map of service name to struct carrying the service runner and name.
	handlers        map[string]ServiceHandler        // map of service name to service handler that will run the service runner methods.
	icStates        intracom.Intracom[ServiceStates] // intracom comms bus for states
	reportAliveSecs uint64                           // system service manager alive report timeout in seconds aka watchdog timeout
	logC            chan DaemonLog                   // log channel for the daemon to log service logs to
	logger          log.Logger                       // logger for the daemon
	started         atomic.Bool                      // flag to indicate if the daemon has been started
	rpcEnabled      bool                             // flag to indicate if the daemon has rpc enabled
	rpcConfig       RPCConfig                        // rpc configuration for the daemon
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(name string, log log.Logger, options ...DaemonOption) Daemon {
	d := &daemon{
		name:            name,
		signals:         []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		services:        make(map[string]DaemonService),
		handlers:        make(map[string]ServiceHandler),
		icStates:        intracom.New[ServiceStates]("rxd-states", log),
		reportAliveSecs: 0,
		logC:            make(chan DaemonLog, 100),
		logger:          log,
		started:         atomic.Bool{},
	}

	for _, option := range options {
		option(d)
	}

	return d

}

func (d *daemon) Start(parent context.Context) error {
	// pre-start checks
	if d.started.Swap(true) {
		return ErrDaemonStarted
	}

	if len(d.services) == 0 {
		return ErrNoServices
	}

	// daemon child context from parent
	dctx, dcancel := context.WithCancel(parent)
	defer dcancel()

	// TODO: To eventually support running rxd on multiple platforms, we need to
	// abstract the notifier to be a part of the daemon configuration.
	// notifier := GetSystemNotifier(ctx) --- probably...
	// For now, we are only supporting linux - systemd.
	// NOTE: Since service manager selection is part of the build runtime.
	// we will probably have to do this via mixture of global and init().
	notifier, err := NewSystemdNotifier(os.Getenv("NOTIFY_SOCKET"), d.reportAliveSecs)
	if err != nil {
		return err
	}

	// Start the notifier, this will start the watchdog portion.
	// so we can notify systemd that we have not hung.
	err = notifier.Start(dctx, d.logC)
	if err != nil {
		return err
	}

	// --- Daemon Service(s) Log Handler ---
	// listens for logs from services via channel and logs them to the daemon logger.
	logDoneC := make(chan struct{})
	go func() {
		// error handler routine
		// closed after the wait group is done.
		for entry := range d.logC {
			d.logger.Log(entry.Level, entry.Message, log.String("service", entry.Name))
		}
		close(logDoneC)
	}()

	// --- Daemon Signal Watcher ---
	// listens for signals to stop the daemon such as OS signals or context done.
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalC)

		select {
		case <-dctx.Done():
			d.logger.Log(log.LevelNotice, "daemon context done", log.String("rxd", "signal-watcher"))
		case <-signalC:
			d.logger.Log(log.LevelNotice, "daemon received signal to stop", log.String("rxd", "signal-watcher"))
		}

		// if we received a signal to stop, cancel the context
		dcancel()

		// inform systemd that we are stopping/cleaning up
		// TODO: Test if this notify should happen before or after cancel()
		// since the watchdog notify continues to until the context is cancelled.
		err = notifier.Notify(NotifyStateStopping)
		if err != nil {
			d.logger.Log(log.LevelError, "error sending 'stopping' notification", log.String("rxd", "systemd-notifier"))
		}

	}()

	// register the states publish topic and channel with the intracom bus
	publishStatesC := make(chan ServiceStates, 1)

	statesTopic, err := d.icStates.Register(internalServiceStates, publishStatesC)
	if err != nil {
		return err
	}

	// --- Service States Watcher ---
	statesDoneC := make(chan struct{})
	stateUpdateC := make(chan StateUpdate, len(d.services)*4)
	// states watcher routine needs to be closed once all services have exited.
	go func(statesC chan<- ServiceStates) {
		states := make(ServiceStates, len(d.services))
		for name := range d.services {
			states[name] = StateExit
		}

		// states watcher routine should be closed after all services have exited.
		for update := range stateUpdateC {
			if current, ok := states[update.Name]; ok && current != update.State {
				d.logger.Log(log.LevelDebug, "service state update", log.String("service", update.Name), log.String("state", update.State.String()))
			}
			// update the state of the service only if it changed.
			states[update.Name] = update.State

			// send the updated states to the intracom bus
			statesC <- states.copy()
		}

		close(statesDoneC)
	}(publishStatesC)

	var dwg sync.WaitGroup
	// launch all services in their own routine.
	for _, service := range d.services {
		dwg.Add(1)

		handler := d.handlers[service.Name]
		if err != nil && handler == nil {
			// TODO: Should we be doing pre-flight checks?
			// is it better to log the error and still try to start the daemon with the services that dont error
			// or is it better to fail fast and exit the daemon with an error?
			d.logger.Log(log.LevelError, "error getting handler for service", log.String("service", service.Name))
			continue
		}

		// each service is handled in its own routine.
		go func(wg *sync.WaitGroup, ctx context.Context, svc DaemonService, h ServiceHandler, stateC chan<- StateUpdate) {
			sctx, scancel := newServiceContextWithCancel(ctx, svc.Name, d.logC, statesTopic)
			// run the service according to the handler policy
			h.Handle(sctx, svc, stateC)
			scancel()
			wg.Done()

		}(&dwg, dctx, service, handler, stateUpdateC)
	}

	// --- Daemon RPC Server ---
	var server *http.Server

	if d.rpcEnabled {
		mux := http.NewServeMux()
		rpcServer := rpc.NewServer()

		cmdHandler := CommandHandler{
			logger: d.logger,
		}

		err := rpcServer.Register(cmdHandler)
		if err != nil {
			// couldnt register the rpc handler, log the error and continue without rpc
			d.logger.Log(log.LevelError, "error registering rpc handler", log.String("rxd", "rpc-server"))
		} else {
			// rpc handlers registered successfully, try to start the rpc server
			addr := d.rpcConfig.Addr + ":" + strconv.Itoa(int(d.rpcConfig.Port))
			mux.Handle("/rpc", rpcServer)
			server = &http.Server{
				Addr:    addr,
				Handler: mux,
			}

			go func(s *http.Server) {
				d.logger.Log(log.LevelInfo, "starting rpc server at "+s.Addr, log.String("rxd", "rpc-server"))
				if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					d.logger.Log(log.LevelError, "error starting rpc server", log.String("rxd", "rpc-server"))
					return
				}
				d.logger.Log(log.LevelInfo, "stopped running and exited successfully", log.String("rxd", "rpc-server"))
			}(server)
		}
	}

	err = notifier.Notify(NotifyStateReady)
	if err != nil {
		d.logger.Log(log.LevelError, "error sending 'ready' notification", log.String("rxd", "systemd-notifier"))
	}
	// block, waiting for all services to exit their lifecycles.
	dwg.Wait()

	// --- Clean up RPC if it was enabled and set ---
	if server != nil {
		timedctx, timedcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer timedcancel()
		if err := server.Shutdown(timedctx); err != nil {
			return err
		}
	}

	d.logger.Log(log.LevelDebug, "cleaning up log and states channels", log.String("rxd", d.name))
	close(stateUpdateC)
	err = statesTopic.Close()
	if err != nil {
		d.logger.Log(log.LevelError, "error closing states topic", log.String("rxd", "intracom"))
	} else {
		d.logger.Log(log.LevelDebug, "states topic closed", log.String("rxd", "intracom"))
	}

	<-statesDoneC // wait for states watcher to finish
	d.logger.Log(log.LevelDebug, "states channel completed", log.String("rxd", d.name))

	close(d.logC) // close the log channel
	<-logDoneC    // wait for log handler to finish
	d.logger.Log(log.LevelDebug, "log channel completed", log.String("rxd", d.name))

	err = d.icStates.Close()
	if err != nil {
		d.logger.Log(log.LevelError, "error closing states intracom", log.String("rxd", "intracom"))
	} else {
		d.logger.Log(log.LevelDebug, "states intracom closed", log.String("rxd", "intracom"))

	}

	// close publishing channel last so subscribers dont try to publish to a closed channel.
	close(publishStatesC)

	d.logger.Log(log.LevelDebug, "intracom closed", log.String("rxd", "intracom"))
	return nil
}

// AddServices adds a list of services to the daemon.
// if any service fails to be added, the error is logged and the next service is attempted.
// any services that fail likely are failing due to name overlap and will be skipped
// if daemon is already started, no new services can be added.
func (d *daemon) AddServices(services ...Service) error {
	for _, service := range services {
		err := d.addService(service)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddService adds a service to the daemon.
// if the service fails to be added, the error will be returned.
func (d *daemon) AddService(service Service) error {
	return d.addService(service)
}

// addService is a helper function to add a service to the daemon.
func (d *daemon) addService(service Service) error {
	if d.started.Load() {
		return ErrAddingServiceOnceStarted
	}

	if service.Name == "" {
		return ErrNoServiceName
	}

	if service.TransitionTimeouts == nil {
		service.TransitionTimeouts = make(StateTransitionTimeouts)
	}

	if service.Handler == nil {
		service.Handler = DefaultHandler
	}

	// NOTE: reflect is being used here only before startup.
	// Since both value structs and pointer structs are allowed to meet an interface
	// the compiler wont catch nil pointer struct with a value receiver.
	// Instead of throwing a panic causing partial startup in Start() later
	// we can pre-flight check the service handler once before hand.
	// Runners can be caught via recover() in their own routines and passed to handler as an error.

	err := checkNilStructPointer(reflect.ValueOf(service.Handler), reflect.TypeOf(service.Handler), "Handle")
	if err != nil {
		return err
	}

	// add the service to the daemon services
	d.services[service.Name] = DaemonService{
		Name:               service.Name,
		Runner:             service.Runner,
		TransitionTimeouts: service.TransitionTimeouts,
	}

	// add the handler to a similar map of service name to handlers
	d.handlers[service.Name] = service.Handler

	return nil
}

func checkNilStructPointer(ival reflect.Value, itype reflect.Type, method string) error {
	if ival.Kind() == reflect.Ptr && ival.IsNil() {
		handlerMethod, _ := itype.Elem().MethodByName(method)
		if handlerMethod.Type.NumIn() > 0 && handlerMethod.Type.In(0).Kind() == reflect.Struct {
			return ErrUninitialized{StructName: itype.String(), Method: method}
		}
	}
	return nil
}
