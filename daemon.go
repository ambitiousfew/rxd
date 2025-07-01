// Package rxd provides a reactive daemon framework for managing concurrent services.
// It allows you to define services, manage their lifecycles, and handle inter-service communication.
// The daemon can be configured with various options such as prestart pipelines, service managers, and logging.
//
// The reactive part of the daemon lies in the callers ability to define custom service runners that can
// react to lifecycles of other services, handle state transitions, and communicate with the daemon.
// The daemon itself reacts to OS signals and is able to gracefully signal shutdown to all services,
// the caller's implementation must be context-aware from within their service runners
// to handle the lifecycle of their services properly.
package rxd

import (
	"context"
	"io"
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

// Daemon is the interface for the reactive daemon.
// It allows you to add services, start the daemon, and manage the lifecycle of the services.
type Daemon interface {
	AddServices(services ...Service) error
	AddService(service Service) error
	Start(ctx context.Context) error
}

// NewDaemon creates and return an instance of the reactive daemon
// NOTE: The service logger runs with a default stdout logger.
// This can be optionally changed by passing the WithServiceLogger option in NewDaemon
// The internal logger is disabled by default and can be enabled by passing the WithInternalLogger option in NewDaemon
func NewDaemon(name string, options ...DaemonOption) Daemon {
	defaultLogger := log.NewLogger(log.LevelInfo, log.NewHandler())

	d := &daemon{
		name:     name,
		signals:  []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		services: make(map[string]DaemonService),
		managers: make(map[string]ServiceManager),
		prestart: &prestartPipeline{
			RestartOnError: true,
			RestartDelay:   5 * time.Second,
			Stages:         []Stage{},
		},
		ic:              intracom.New("rxd-intracom"),
		reportAliveSecs: 0,
		logWorkerCount:  2,
		serviceLogger:   defaultLogger,
		// by default the internal daemon logger is disabled.
		internalLogger: log.NewLogger(log.LevelDebug, &daemonLogHandler{
			filepath: "rxd.log",        // relative to the executable, if enabled
			enabled:  false,            // disabled by default
			total:    0,                // total bytes written to the log file
			limit:    10 * 1024 * 1024, // 10MB (if enabled)
			file:     nil,
			mu:       sync.RWMutex{},
		}),
		started: atomic.Bool{},
	}

	for _, option := range options {
		option(d)
	}

	return d
}

type daemon struct {
	name            string                    // name of the daemon will be used in logging
	signals         []os.Signal               // OS signals you want your daemon to listen for
	services        map[string]DaemonService  // map of service name to struct carrying the service runner and name.
	managers        map[string]ServiceManager // map of service name to service handler that will run the service runner methods.
	prestart        Pipeline                  // prestart pipeline to run before starting the daemon services
	ic              *intracom.Intracom        // intracom registry for the daemon to communicate with services
	reportAliveSecs uint64                    // system service manager alive report timeout in seconds aka watchdog timeout
	logWorkerCount  int                       // number of concurrent log workers used to receive and write service logs (default: 2)
	serviceLogger   log.Logger                // logger used by user services
	internalLogger  log.Logger                // logger for the internal daemon, debugging
	started         atomic.Bool               // flag to indicate if the daemon has been started
	rpcEnabled      bool                      // flag to indicate if the daemon has rpc enabled
	rpcConfig       RPCConfig                 // rpc configuration for the daemon
}

func (d *daemon) Start(parent context.Context) error {
	// pre-start checks
	if d.started.Swap(true) {
		return ErrDaemonStarted
	}

	if len(d.services) == 0 {
		return ErrNoServices
	}

	nameField := log.String("rxd", d.name)

	// daemon child context from parent
	dctx, dcancel := context.WithCancel(parent)
	defer dcancel()

	// --- Start the Daemon Service Log Watcher ---
	// listens for logs from services via channel and logs them to the daemon logger.

	// --- Daemon Signal Watcher ---
	// listens for signals to stop the daemon such as OS signals or context done.
	go func() {
		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(signalC)

		select {
		case <-dctx.Done():
			d.internalLogger.Log(log.LevelDebug, "signal watcher received context done from parent context", nameField)
		case sig := <-signalC:
			d.internalLogger.Log(log.LevelNotice, "signal watcher received an os signal", log.String("signal", sig.String()), nameField)
			// if we received a signal to stop, cancel the context
			dcancel()
		}
	}()

	// --- Prestart Pipeline ---
	// run all prestart checks in order
	errC := d.prestart.Run(dctx)
	for err := range errC {
		// logC <- err
		d.internalLogger.Log(log.LevelError, "error in prestart pipeline", log.String("error", err.Message), nameField)
	}

	d.internalLogger.Log(log.LevelDebug, "creating intracom topic", log.String("topic", internalServiceStates), nameField)
	statesTopic, err := intracom.CreateTopic[ServiceStates](d.ic, intracom.TopicConfig{
		Name: internalServiceStates,
		// Buffer:      1,
		ErrIfExists: true,
	})

	if err != nil {
		d.internalLogger.Log(log.LevelError, "error creating intracom topic", log.Error("error", err), nameField)
		return err
	}

	stateUpdateC := make(chan ServiceStateUpdate, len(d.services)*4)

	// --- Service States Watcher ---
	// states watcher routine needs to be closed once all services have exited.
	d.internalLogger.Log(log.LevelInfo, "starting service states watcher", nameField)
	statesDoneC := d.statesWatcher(statesTopic, stateUpdateC)

	d.internalLogger.Log(log.LevelInfo, "starting "+strconv.Itoa(len(d.services))+" services", nameField)
	var dwg sync.WaitGroup // daemon wait group

	// --- Launch Daemon Service(s) ---
	// launch all services in their own routine.
	for _, service := range d.services {
		manager, ok := d.managers[service.Name]
		if !ok {
			// TODO: Should we be doing pre-flight checks?
			// is it better to log the error and still try to start the daemon with the services that dont error
			// or is it better to fail fast and exit the daemon with an error?
			d.internalLogger.Log(log.LevelError, "error getting manager for service", log.String("service_name", service.Name), nameField)
			continue
		}

		dwg.Add(1)
		// each service is handled in its own routine.
		go func(ctx context.Context, wg *sync.WaitGroup, ds DaemonService, manager ServiceManager, stateC chan<- ServiceStateUpdate) {
			sctx, scancel := newServiceContextWithCancel(ctx, ds.Name, d.serviceLogger, d.ic)

			defer func() {
				// recover from any panics in the service runner
				// no service should be able to crash the daemon.
				if r := recover(); r != nil {
					d.serviceLogger.Log(log.LevelError, "recovered from panic", log.String("service", ds.Name), log.Any("error", r))
					d.internalLogger.Log(log.LevelError, "recovered from panic", log.String("service_name", ds.Name), log.Any("error", r), nameField)
					// NOTE: the final state of the service should be "entering" the exit state.
					stateC <- ServiceStateUpdate{Name: ds.Name, State: StateExit, Transition: TransitionEntering}
				}
				scancel()
				wg.Done()
				d.internalLogger.Log(log.LevelInfo, "service has exited", log.String("service_name", ds.Name), nameField)
			}()

			d.internalLogger.Log(log.LevelInfo, "starting service", log.String("service_name", ds.Name), nameField)

			// run the service according to the manager policy
			manager.Manage(sctx, ds, stateC) // blocking call until the service exits its lifecycle.

		}(dctx, &dwg, service, manager, stateUpdateC)
	}

	// --- Daemon RPC Server ---
	var server *http.Server

	if d.rpcEnabled {
		mux := http.NewServeMux()
		rpcServer := rpc.NewServer()

		cmdHandler := CommandHandler{
			sLogger: d.serviceLogger,
			iLogger: d.internalLogger,
		}

		err := rpcServer.Register(cmdHandler)
		if err != nil {
			// couldnt register the rpc handler, log the error and continue without rpc
			d.internalLogger.Log(log.LevelError, "error registering rpc handler", nameField)
		} else {
			// rpc handlers registered successfully, try to start the rpc server
			addr := d.rpcConfig.Addr + ":" + strconv.Itoa(int(d.rpcConfig.Port))
			mux.Handle("/rpc", rpcServer)
			server = &http.Server{
				Addr:    addr,
				Handler: mux,
			}

			go func(s *http.Server) {
				d.internalLogger.Log(log.LevelInfo, "starting rpc server at "+s.Addr, nameField)
				if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					d.internalLogger.Log(log.LevelError, "error starting rpc server", nameField)
					return
				}
				d.internalLogger.Log(log.LevelInfo, "stopped running rpc server and exited successfully", nameField)
			}(server)
		}
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

	d.internalLogger.Log(log.LevelDebug, "closing states watcher", nameField)
	// since all services have exited their lifecycles, we can close the states update channel.
	close(stateUpdateC)
	<-statesDoneC // wait for states watcher to finish
	d.internalLogger.Log(log.LevelDebug, "states watcher closed", nameField)

	d.internalLogger.Log(log.LevelDebug, "closing intracom", nameField)
	// TODO: these logs should not be interleaved with the user service logs.
	err = intracom.Close(d.ic)
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error closing intracom", log.Error("error", err), nameField)
	} else {
		d.internalLogger.Log(log.LevelDebug, "intracom closed", nameField)
	}

	d.internalLogger.Log(log.LevelDebug, "services log channel closed", nameField)

	// if the internal logger is an io.Closer, close it.
	if internalLogger, ok := d.internalLogger.(io.Closer); ok {
		internalLogger.Close()
	}
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

func (d *daemon) statesWatcher(statesTopic intracom.Topic[ServiceStates], stateUpdatesC <-chan ServiceStateUpdate) <-chan struct{} {
	doneC := make(chan struct{})

	go func() {
		// retrieve the publisher channel for the states topic
		d.internalLogger.Log(log.LevelDebug, "states topic publish channel", log.String("topic", internalServiceStates))
		statesC := statesTopic.PublishChannel()

		states := make(ServiceStates, len(d.services))
		for name := range d.services {
			// initialize the state of each service to init
			states[name] = StateUpdate{State: StateUnknown, Transition: TransitionEntering}
		}

		// states watcher routine should be closed after all services have exited.
		for state := range stateUpdatesC {
			d.internalLogger.Log(log.LevelDebug, "----- states transition update", log.String("service_name", state.Name), log.String("state", state.State.String()))
			// update the state of the service only if it changed.
			states[state.Name] = StateUpdate{
				State:      state.State,
				Transition: state.Transition,
			}

			// send the updated states to the intracom bus
			statesC <- states.copy()
		}
		d.internalLogger.Log(log.LevelDebug, "states watcher completed")
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
