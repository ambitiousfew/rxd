package rxd

import (
	"context"
	"errors"
	"io"
	"net/http"
	gorpc "net/rpc"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ambitiousfew/rxd/config"
	"github.com/ambitiousfew/rxd/intracom"
	"github.com/ambitiousfew/rxd/log"
	"github.com/ambitiousfew/rxd/pkg/rpc"
	"github.com/ambitiousfew/rxd/sysctl"
)

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
	internalLogger := log.NewLogger(log.LevelDebug, &daemonLogHandler{
		filepath: "rxd.log",        // relative to the executable, if enabled
		enabled:  false,            // disabled by default
		total:    0,                // total bytes written to the log file
		limit:    10 * 1024 * 1024, // 10MB (if enabled)
		file:     nil,
		mu:       sync.RWMutex{},
	}).With(log.String("rxd", name))

	// construct a default system agent with a default internal logger
	defaultAgent := sysctl.NewDefaultSystemAgent(sysctl.WithCustomLogger(internalLogger))

	// construct the daemon with reasonable default values
	d := &daemon{
		name:           name,
		services:       make(map[string]Service),
		serviceRelays:  make(map[string]chan rpc.CommandSignal),
		agent:          defaultAgent,
		ic:             intracom.New("rxd-intracom"),
		logWorkerCount: 2,
		serviceLogger:  defaultLogger,
		// by default the internal daemon logger is disabled.
		internalLogger: internalLogger,
		started:        atomic.Bool{},
		configuration:  noopConfigReadLoader{},
		loggingC:       make(chan DaemonLog, 50),
	}

	// apply any optional overrides to the daemon
	for _, option := range options {
		option(d)
	}

	return d
}

type daemon struct {
	name           string                            // name of the daemon will be used in logging
	configuration  config.ReadLoader                 // configuration reader and loader for the daemon
	services       map[string]Service                // map of service name to struct carrying the service runner and name.
	serviceRelays  map[string]chan rpc.CommandSignal // map of service name to channel to relay command signals to the service
	agent          sysctl.Agent                      // daemon agent that interacts with the OS specific system service manager
	ic             *intracom.Intracom                // intracom registry for the daemon to communicate with services
	logWorkerCount int                               // number of concurrent log workers used to receive and write service logs (default: 2)
	serviceLogger  log.Logger                        // logger used by user services
	internalLogger log.Logger                        // logger for the internal daemon, debugging
	started        atomic.Bool                       // flag to indicate if the daemon has been started
	rpcEnabled     bool                              // flag to indicate if the daemon has rpc enabled
	rpcConfig      RPCConfig                         // rpc configuration for the daemon
	loggingC       chan DaemonLog
}

func (d *daemon) Start(parent context.Context) error {
	// pre-start checks
	if d.started.Swap(true) {
		return ErrDaemonStarted
	}

	if len(d.services) == 0 {
		return ErrNoServices
	}

	if d.ic == nil {
		return ErrNoIntracomBus
	}

	err := d.configuration.Read(parent)
	if err != nil {
		return err
	}

	// daemon child context from parent
	daemonCtx, daemonCancel := context.WithCancel(parent)
	defer daemonCancel()

	// call the platform agent Run method
	go func() {
		d.internalLogger.Log(log.LevelDebug, "starting platform agent")
		err := d.agent.Run(parent)
		if err != nil {
			d.internalLogger.Log(log.LevelError, "error running platform agent", log.Error("error", err))
		}
	}()

	// --- Start the Daemon Service Log Watcher ---
	// listens for logs from services via channel and logs them to the daemon logger.
	loggerDoneC := d.serviceLogWatcher(d.loggingC)

	d.internalLogger.Log(log.LevelDebug, "creating intracom topic", log.String("topic", internalServiceStates))
	statesTopic, err := intracom.CreateTopic[ServiceStates](d.ic, intracom.TopicConfig{
		Name:        internalServiceStates,
		ErrIfExists: true,
	})

	if err != nil {
		d.internalLogger.Log(log.LevelError, "error creating intracom topic", log.Error("error", err))
		return err
	}

	// --- Setup the RPC Server (if option was enabled) ---
	cmdTopic, err := intracom.CreateTopic[RPCCommandRequest](d.ic, intracom.TopicConfig{
		Name:        internalCommandSignals,
		ErrIfExists: true,
	})

	if err != nil {
		d.internalLogger.Log(log.LevelError, "error creating intracom topic", log.Error("error", err))
		return err
	}

	rpcServer, err := d.startRPCServer(cmdTopic)
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error starting rpc server", log.Error("error", err))
		return err
	}

	stateUpdateC := make(chan StateUpdate, len(d.services)*4)
	// --- Service States Watcher ---
	// states watcher routine needs to be closed once all services have exited.
	d.internalLogger.Log(log.LevelInfo, "starting service states watcher")
	statesDoneC := d.statesWatcher(statesTopic, stateUpdateC)

	// --- Service Configuration Watcher ---
	configUpdateTopic, err := intracom.CreateTopic[int64](d.ic, intracom.TopicConfig{
		Name:        internalConfigUpdate,
		ErrIfExists: true,
	})
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error creating intracom topic", log.Error("error", err))
		return err
	}

	d.internalLogger.Log(log.LevelInfo, "starting "+strconv.Itoa(len(d.services))+" services")
	var wg sync.WaitGroup // daemon wait group
	// --- Launch Daemon Service(s) ---
	// launch all services in their own routine.
	for _, service := range d.services {
		wg.Add(1)
		go d.startManager(daemonCtx, &wg, service, stateUpdateC)
	}

	configUpdateC := configUpdateTopic.PublishChannel()
	// daemon agent watches for signals and acts accordingly
	// blocks until the daemon context is cancelled or a signal (interrupt or terminate) is received
	for signal := range d.agent.WatchForSignals(daemonCtx) {
		d.internalLogger.Log(log.LevelNotice, "received agent signal", log.String("signal", signal.String()))
		switch signal {
		case sysctl.SignalReloading:
			d.internalLogger.Log(log.LevelDebug, "reloading configuration", log.String("signal", signal.String()))
			// TODO: perform actual reload operation
			err := d.configuration.Read(daemonCtx)
			if err != nil {
				d.internalLogger.Log(log.LevelError, "error reloading configuration", log.Error("error", err))
				continue
			}

			select {
			case <-daemonCtx.Done():
				break
			case configUpdateC <- time.Now().Unix():
				// sent the current time as a signal to all services who care.
			}

			err = d.agent.Notify(sysctl.NotifyReloaded)
			if err != nil {
				d.internalLogger.Log(log.LevelError, "error notifying system service manager of reload", log.Error("error", err))
			}

		case sysctl.SignalStarting:
			err = d.agent.Notify(sysctl.NotifyRunning)
			if err != nil {
				d.internalLogger.Log(log.LevelError, "error notifying system service manager of starting", log.Error("error", err))
			}
		case sysctl.SignalStopping, sysctl.SignalRestarting:
			err = d.agent.Notify(sysctl.NotifyStopping)
			if err != nil {
				d.internalLogger.Log(log.LevelError, "error notifying system service manager of stopping", log.Error("error", err))
			}
			daemonCancel()

		default:
			d.internalLogger.Log(log.LevelNotice, "received ignored signal", log.String("signal", signal.String()))
			continue
		}
	}

	// Continue to block until all services have exited.
	wg.Wait()

	// ALL SERVICE MANAGERS HAVE EXITED THEIR LIFECYCLES
	//   CLEANUP AND SHUTDOWN

	// --- Clean up RPC Server it was created ---
	if rpcServer != nil {
		timedctx, timedcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer timedcancel()
		if err := rpcServer.Shutdown(timedctx); err != nil {
			return err
		}
	}

	// no services should be reporting their states anymore
	d.internalLogger.Log(log.LevelInfo, "closing states watcher")
	close(stateUpdateC) // close the states update channel
	<-statesDoneC       // wait for states watcher (routine) to exit

	d.internalLogger.Log(log.LevelInfo, "closing intracom")
	err = intracom.Close(d.ic)
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error closing intracom", log.Error("error", err))
	}

	d.internalLogger.Log(log.LevelInfo, "closing services log channel")
	// close the services log channel to signal the log watcher to finish
	close(d.loggingC) // signal close the log channel
	<-loggerDoneC     // wait for log watcher to finish

	// if the internal logger is an io.Closer, close it.
	if internalLogger, ok := d.internalLogger.(io.Closer); ok {
		internalLogger.Close()
	}

	// finally notify the system service manager that the daemon has stopped
	err = d.agent.Notify(sysctl.NotifyStopped)
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error notifying system service manager", log.Error("error", err))
	}

	d.internalLogger.Log(log.LevelInfo, "closing platform agent")
	err = d.agent.Close()
	if err != nil {
		d.internalLogger.Log(log.LevelError, "error closing system service manager", log.Error("error", err))
	}

	d.internalLogger.Log(log.LevelInfo, "daemon successfully exited")
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
	d.services[service.Name] = service
	return nil
}

func (d *daemon) startManager(ctx context.Context, wg *sync.WaitGroup, service Service, updateStateC chan<- StateUpdate) {
	defer func() {
		// recover from any panics in the service runner
		// no service should be able to crash the daemon.
		if r := recover(); r != nil {
			d.serviceLogger.Log(log.LevelError, "recovered from panic", log.String("service", service.Name), log.Any("error", r))
			d.internalLogger.Log(log.LevelError, "recovered from panic", log.String("service_name", service.Name), log.Any("error", r))
		}
		updateStateC <- StateUpdate{Name: service.Name, State: StateExit}
		d.internalLogger.Log(log.LevelInfo, "service has been stopped", log.String("service_name", service.Name))
		wg.Done()
	}()

	relayC := d.serviceRelays[service.Name]
	consumerName := "_manager-" + service.Name

	var configSub <-chan int64
	var err error
	// if the service runner implements the ServiceReloader interface, subscribe to the config update topic
	if _, ok := service.Runner.(ServiceLoader); ok {
		configSub, err = intracom.CreateSubscription(ctx, d.ic, internalConfigUpdate, 0, intracom.SubscriberConfig[int64]{
			ConsumerGroup: consumerName,
			ErrIfExists:   false,
			BufferSize:    1,
			BufferPolicy:  intracom.BufferPolicyDropNewest[int64]{},
		})
		if err != nil {
			d.internalLogger.Log(log.LevelError, "error creating intracom subscription", log.Error("error", err))
			return
		}
		defer intracom.RemoveSubscription(d.ic, internalConfigUpdate, consumerName, configSub)
	}

	dstate := DaemonState{
		loader:  d.configuration,
		logC:    d.loggingC,
		ic:      d.ic,
		configC: configSub,
		updateC: updateStateC,
		logger:  d.internalLogger,
	}

	msvc := ManagedService{
		Name:     service.Name,
		Runner:   service.Runner,
		CommandC: relayC,
	}

	d.internalLogger.Log(log.LevelInfo, "manager starting service", log.String("service_name", service.Name))
	// run the service according to the manager policy
	service.Manager.Manage(ctx, dstate, msvc)
}

func (d *daemon) serviceLogWatcher(logC <-chan DaemonLog) <-chan struct{} {
	doneC := make(chan struct{})

	go func() {
		// semaphore to limit the number of concurrent log writes to the daemon logger.
		sema := make(chan struct{}, d.logWorkerCount)
		for entry := range logC {
			sema <- struct{}{}
			go func() {
				d.serviceLogger.Log(entry.Level, entry.Message, entry.Fields...)
				<-sema
			}()
		}
		close(doneC)
	}()

	return doneC
}

func (d *daemon) statesWatcher(statesTopic intracom.Topic[ServiceStates], stateUpdatesC <-chan StateUpdate) <-chan struct{} {
	doneC := make(chan struct{})

	go func() {
		// retrieve the publisher channel for the states topic
		d.internalLogger.Log(log.LevelDebug, "starting states watcher", log.String("topic", internalServiceStates))
		statesC := statesTopic.PublishChannel()

		states := make(ServiceStates, len(d.services))
		for name := range d.services {
			states[name] = StateExit
		}

		// states watcher routine should be closed after all services have exited.
		for state := range stateUpdatesC {
			d.internalLogger.Log(log.LevelDebug, "states transition update", log.String("service_name", state.Name), log.String("state", state.State.String()))
			states[state.Name] = state.State
			// send the updated states to the intracom bus
			statesC <- states.copy()
		}
		d.internalLogger.Log(log.LevelDebug, "states watcher completed")
		// signal done after states watcher has finished.
		close(doneC)
	}()

	return doneC
}

func (d *daemon) startRPCServer(commandTopic intracom.Topic[RPCCommandRequest]) (*http.Server, error) {
	var server *http.Server
	if !d.rpcEnabled {
		return server, nil
	}

	mux := http.NewServeMux()
	rpcServer := gorpc.NewServer()

	// copy the services map to a new map
	for name := range d.services {
		d.serviceRelays[name] = make(chan rpc.CommandSignal)
	}

	rpcCmdHandler, err := NewCommandHandler(CommandHandlerConfig{
		Services:            d.serviceRelays,
		CommandRequestTopic: commandTopic,
		Logger:              d.serviceLogger,
	})
	if err != nil {
		return nil, err
	}
	defer rpcCmdHandler.close()

	err = rpcServer.Register(rpcCmdHandler)
	if err != nil {
		// couldnt register the rpc handler, log the error and continue without rpc
		d.internalLogger.Log(log.LevelError, "error registering rpc handler")
		return server, errors.New("error registering rpc handler")
	} else {
		// rpc handlers registered successfully, try to start the rpc server
		addr := d.rpcConfig.Addr + ":" + strconv.Itoa(int(d.rpcConfig.Port))
		mux.Handle("/rpc", rpcServer)
		server = &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		go func(s *http.Server) {
			// launch RPC command server
			d.internalLogger.Log(log.LevelInfo, "starting rpc server at "+s.Addr)
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				d.internalLogger.Log(log.LevelError, "error starting rpc server")
				return
			}
			d.internalLogger.Log(log.LevelInfo, "stopped running rpc server and exited successfully")
		}(server)
	}

	return server, nil
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

type noopConfigReadLoader struct{}

func (noopConfigReadLoader) Read(context.Context) error           { return nil }
func (noopConfigReadLoader) Load(context.Context) ([]byte, error) { return nil, nil }
