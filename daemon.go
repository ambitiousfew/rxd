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
	name            string                    // name of the daemon will be used in logging
	signals         []os.Signal               // OS signals you want your daemon to listen for
	services        map[string]DaemonService  // map of service name to struct carrying the service runner and name.
	managers        map[string]ServiceManager // map of service name to service handler that will run the service runner methods.
	ic              *intracom.Intracom        // intracom registry for the daemon to communicate with services
	reportAliveSecs uint64                    // system service manager alive report timeout in seconds aka watchdog timeout
	logC            chan DaemonLog            // log channel for the daemon to log service logs to
	serviceLogger   log.Logger                // logger used by user services
	internalLogger  log.Logger                // logger for the internal daemon, debugging
	started         atomic.Bool               // flag to indicate if the daemon has been started
	rpcEnabled      bool                      // flag to indicate if the daemon has rpc enabled
	rpcConfig       RPCConfig                 // rpc configuration for the daemon
}

// NewDaemon creates and return an instance of the reactive daemon
func NewDaemon(name string, logger log.Logger, options ...DaemonOption) Daemon {
	d := &daemon{
		name:            name,
		signals:         []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		services:        make(map[string]DaemonService),
		managers:        make(map[string]ServiceManager),
		ic:              intracom.New("rxd-intracom"),
		reportAliveSecs: 0,
		logC:            make(chan DaemonLog, 100),
		serviceLogger:   logger,
		// by default the internal daemon logger is disabled.
		internalLogger: log.NewLogger(log.LevelDebug, noopLogHandler{}),
		started:        atomic.Bool{},
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

	// --- Service Manager Notifier ---
	// TODO:: Future work here will be to support multiple platform service managers
	// such as windows service manager, systemd, etc.
	//
	// This will require manager selection to be selected dynamically at runtime.
	// notifier := GetSystemNotifier(ctx) --- probably...
	// For now, we are only supporting linux - systemd.
	notifier, err := NewSystemdNotifier(os.Getenv("NOTIFY_SOCKET"), d.reportAliveSecs)
	if err != nil {
		return err
	}

	// Start the notifier, this will start the watchdog portion.
	// so we can notify systemd that we have not hung.
	err = notifier.Start(dctx, d.internalLogger)
	if err != nil {
		return err
	}

	// --- Start the Daemon Service Log Watcher ---
	// listens for logs from services via channel and logs them to the daemon logger.
	loggerDoneC := d.serviceLogWatcher(d.logC)

	// --- Daemon Signal Watcher ---
	// listens for signals to stop the daemon such as OS signals or context done.
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalC)

		select {
		case <-dctx.Done():
			d.internalLogger.Log(log.LevelNotice, "daemon context done", log.String("rxd", "signal-watcher"))
		case <-signalC:
			d.internalLogger.Log(log.LevelNotice, "daemon received signal to stop", log.String("rxd", "signal-watcher"))
		}

		// if we received a signal to stop, cancel the context
		dcancel()

		// inform systemd that we are stopping/cleaning up
		// TODO: Test if this notify should happen before or after cancel()
		// since the watchdog notify continues to until the context is cancelled.
		err := notifier.Notify(NotifyStateStopping)
		if err != nil {
			d.internalLogger.Log(log.LevelError, "error sending 'stopping' notification", log.String("rxd", "systemd-notifier"))
		}

	}()

	statesTopic, err := intracom.CreateTopic[ServiceStates](d.ic, intracom.TopicConfig{
		Name: internalServiceStates,
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		return err
	}

	stateUpdateC := make(chan StateUpdate, len(d.services)*4)

	// --- Service States Watcher ---
	// states watcher routine needs to be closed once all services have exited.
	statesDoneC := d.statesWatcher(statesTopic, stateUpdateC)

	// --- Launch Daemon Service(s) ---
	var dwg sync.WaitGroup
	// launch all services in their own routine.
	for _, service := range d.services {
		dwg.Add(1)

		manager, ok := d.managers[service.Name]
		if !ok {
			// TODO: Should we be doing pre-flight checks?
			// is it better to log the error and still try to start the daemon with the services that dont error
			// or is it better to fail fast and exit the daemon with an error?
			d.internalLogger.Log(log.LevelError, "error getting manager for service", log.String("service_name", service.Name))
			continue
		}

		// each service is handled in its own routine.
		go func(wg *sync.WaitGroup, ctx context.Context, ds DaemonService, manager ServiceManager, stateC chan<- StateUpdate) {
			d.internalLogger.Log(log.LevelDebug, "starting service", log.String("service_name", ds.Name))
			sctx, scancel := newServiceContextWithCancel(ctx, ds.Name, d.logC, d.ic)
			// run the service according to the manager policy
			manager.Manage(sctx, ds, func(service string, state State) {
				stateC <- StateUpdate{Name: service, State: state}
			})
			scancel()
			wg.Done()

		}(&dwg, dctx, service, manager, stateUpdateC)
	}

	// --- Daemon RPC Server ---
	var server *http.Server

	if d.rpcEnabled {
		mux := http.NewServeMux()
		rpcServer := rpc.NewServer()

		cmdHandler := CommandHandler{
			logger: d.internalLogger,
		}

		err := rpcServer.Register(cmdHandler)
		if err != nil {
			// couldnt register the rpc handler, log the error and continue without rpc
			d.internalLogger.Log(log.LevelError, "error registering rpc handler", log.String("rxd", "rpc-server"))
		} else {
			// rpc handlers registered successfully, try to start the rpc server
			addr := d.rpcConfig.Addr + ":" + strconv.Itoa(int(d.rpcConfig.Port))
			mux.Handle("/rpc", rpcServer)
			server = &http.Server{
				Addr:    addr,
				Handler: mux,
			}

			go func(s *http.Server) {
				d.internalLogger.Log(log.LevelInfo, "starting rpc server at "+s.Addr, log.String("rxd", "rpc-server"))
				if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					d.internalLogger.Log(log.LevelError, "error starting rpc server", log.String("rxd", "rpc-server"))
					return
				}
				d.internalLogger.Log(log.LevelInfo, "stopped running and exited successfully", log.String("rxd", "rpc-server"))
			}(server)
		}
	}

	err = notifier.Notify(NotifyStateReady)
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error sending 'ready' notification", log.String("rxd", "systemd-notifier"))
	}

	// block until all services have exited their lifecycles
	dwg.Wait()

	// -- ALL SERVICES HAVE EXITED THEIR LIFECYCLES --
	//         CLEANUP AND SHUTDOWN

	// --- Clean up RPC if it was enabled and set ---
	if server != nil {
		timedctx, timedcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer timedcancel()
		if err := server.Shutdown(timedctx); err != nil {
			return err
		}
	}

	// since all services have exited their lifecycles, we can close the states update channel.
	close(stateUpdateC)
	<-statesDoneC // wait for states watcher to finish
	d.internalLogger.Log(log.LevelDebug, "states channel completed", log.String("rxd", d.name))

	// TODO: these logs should not be interleaved with the user service logs.
	err = intracom.Close(d.ic)
	// err = d.icStates.Close()
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error closing intracom", log.String("rxd", "intracom"))
	}

	close(d.logC) // signal close the log channel
	<-loggerDoneC // wait for log watcher to finish

	d.internalLogger.Log(log.LevelDebug, "log channel completed", log.String("rxd", d.name))

	d.internalLogger.Log(log.LevelDebug, "intracom closed", log.String("rxd", "intracom"))
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

	if service.Manager == nil {
		service.Manager = NewDefaultManager()
	}

	// NOTE: reflect is being used here only before startup.
	// Since both value structs and pointer structs are allowed to meet an interface
	// the compiler wont catch nil pointer struct with a value receiver.
	// Instead of throwing a panic causing partial startup in Start() later
	// we can pre-flight check the service handler once before hand.
	// Runners can be caught via recover() in their own routines and passed to handler as an error.

	err := checkNilStructPointer(reflect.ValueOf(service.Manager), reflect.TypeOf(service.Manager), "Manage")
	if err != nil {
		return err
	}

	// add the service to the daemon services
	d.services[service.Name] = DaemonService{
		Name:   service.Name,
		Runner: service.Runner,
	}

	// add the handler to a similar map of service name to handlers
	d.managers[service.Name] = service.Manager

	return nil
}

func (d *daemon) serviceLogWatcher(logC <-chan DaemonLog) <-chan struct{} {
	doneC := make(chan struct{})

	go func() {
		for entry := range logC {
			d.serviceLogger.Log(entry.Level, entry.Message, entry.Fields...)
		}
		close(doneC)
	}()

	return doneC
}
func (d *daemon) statesWatcher(statesTopic intracom.Topic[ServiceStates], stateUpdatesC <-chan StateUpdate) <-chan struct{} {
	doneC := make(chan struct{})

	go func() {
		// retrieve the publisher channel for the states topic
		statesC := statesTopic.PublishChannel()

		states := make(ServiceStates, len(d.services))
		for name := range d.services {
			states[name] = StateExit
		}

		// states watcher routine should be closed after all services have exited.
		for state := range stateUpdatesC {
			// if current, ok := states[state.Name]; ok && current != state.State {
			// TODO: daemon internal logs like this should probably get their own logger like intracom.
			// we dont really want these logs interleaved with the user service logs.
			// d.logger.Log(log.LevelDebug, "service state update", log.String("service_name", state.Name), log.String("state", state.State.String()))
			// }
			// update the state of the service only if it changed.
			states[state.Name] = state.State

			// send the updated states to the intracom bus
			statesC <- states.copy()
		}

		// signal done after states watcher has finished.
		close(doneC)
	}()

	return doneC
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
