//go:build windows

package sysctl

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/ambitiousfew/rxd/log"
)

var _ Agent = (*winscmAgent)(nil)

// Windows API and SCM functions.
var (
	modAdvapi32                       = syscall.NewLazyDLL("advapi32.dll")                   // contains Windows Service-related functions.
	procStartServiceCtrlDispatcherW   = modAdvapi32.NewProc("StartServiceCtrlDispatcherW")   // registers service with SCM and blocks until serviceMain exits.
	procRegisterServiceCtrlHandlerExW = modAdvapi32.NewProc("RegisterServiceCtrlHandlerExW") // registers a function "serviceCtrlHandler" to handle control codes.
	procSetServiceStatus              = modAdvapi32.NewProc("SetServiceStatus")              // updates SCM with the current service status.
)

// Constants for service states and control codes.
const (
	SERVICE_WIN32_OWN_PROCESS     = 0x00000010 // dwServiceType
	SERVICE_ACCEPT_STOP           = 0x00000001 // dwControlsAccepted
	SERVICE_ACCEPT_PAUSE_CONTINUE = 0x00000002 // dwControlsAccepted
	SERVICE_ACCEPT_SHUTDOWN       = 0x00000004 // dwControlsAccepted
	SERVICE_ACCEPT_PARAMCHANGE    = 0x00000008 // dwControlsAccepted
	SERVICE_CONTROL_STOP          = 0x00000001 // Service control code
	SERVICE_CONTROL_PAUSE         = 0x00000002 // Service control code
	SERVICE_CONTROL_CONTINUE      = 0x00000003 // Service control code
	SERVICE_CONTROL_INTERROGATE   = 0x00000004 // Service control code
	SERVICE_CONTROL_SHUTDOWN      = 0x00000005 // Service control code
	SERVICE_CONTROL_PARAMCHANGE   = 0x00000006 // Service control code
	SERVICE_STOPPED               = 0x00000001 // dwCurrentState
	SERVICE_START_PENDING         = 0x00000002 // dwCurrentState
	SERVICE_STOP_PENDING          = 0x00000003 // dwCurrentState
	SERVICE_RUNNING               = 0x00000004 // dwCurrentState
	SERVICE_CONTINUE_PENDING      = 0x00000005 // dwCurrentState
	SERVICE_PAUSE_PENDING         = 0x00000006 // dwCurrentState
	SERVICE_PAUSED                = 0x00000007 // dwCurrentState
)

// serviceStatus is a type representing the current status of the service.
// dw implies (dword) is a 32-bit unsigned integer.
// this struct maps to the binary memory layout of the Windows API SERVICE_STATUS structure.
// it must be in exact order and size to be compatible with the Windows API.
type serviceStatus struct {
	dwServiceType             uint32 // Service type (e.g., SERVICE_WIN32_OWN_PROCESS)
	dwCurrentState            uint32 // Current service state (START_PENDING, RUNNING, etc.)
	dwControlsAccepted        uint32 // What controls are accepted (STOP, SHUTDOWN, etc.)
	dwWin32ExitCode           uint32 // Exit code (use 0 for normal, 1 for error)
	dwServiceSpecificExitCode uint32 // Use this instead of dwWin32ExitCode if specific error codes are needed
	dwCheckPoint              uint32 // Progress tracking for START_PENDING, STOP_PENDING
	dwWaitHint                uint32 // How long SCM should wait before assuming failure
}

// serviceTableEntry mirrors the Windows API SERVICE_TABLE_ENTRY structure.
type serviceTableEntry struct {
	serviceName *uint16
	serviceProc uintptr
}

func NewWindowsSCMAgent(serviceName string, opt ...WindowsSCMAgentOption) (*winscmAgent, error) {
	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return nil, err
	}

	agent := &winscmAgent{
		serviceName: serviceNamePtr,
		signalC:     make(chan SignalState),
		notifyC:     make(chan NotifyState),
		logger:      noopLogger{},
		running:     atomic.Bool{},
	}

	for _, o := range opt {
		o(agent)
	}

	return agent, nil
}

type winscmAgent struct {
	serviceName *uint16
	signalC     chan SignalState
	notifyC     chan NotifyState
	logger      log.Logger
	running     atomic.Bool
}

func (a *winscmAgent) SetLogger(logger log.Logger) {
	a.logger = logger
}

func (a *winscmAgent) WatchForSignals(ctx context.Context) <-chan SignalState {
	stateC := make(chan SignalState, 1)
	go func() {
		defer close(stateC)

		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, syscall.SIGTERM, os.Kill)
		defer signal.Stop(signalC)

		a.logger.Log(log.LevelDebug, "watching for signals...")

		for {
			select {
			case <-ctx.Done():
				return
			case <-signalC:
				stateC <- SignalStopping
				a.logger.Log(log.LevelDebug, "received signal: SIGTERM")
			case sig, open := <-a.signalC:
				if !open {
					return
				}

				a.logger.Log(log.LevelDebug, "received signal: "+sig.String())

				switch sig {
				case SignalStarting:
					// Triggered by Run() before serviceMain is called (blocks).
					stateC <- SignalStarting
				case SignalStopping, SignalRestarting:
					// trigerred by a SERVICE_CONTROL_STOP signal
					stateC <- SignalStopping
				case SignalReloading:
					// triggered by a SERVICE_CONTROL_PARAMCHANGE signal
					stateC <- SignalReloading
				case SignalAlive:
					// triggered by a SERVICE_CONTROL_INTERROGATE signal
					stateC <- SignalAlive
				default:
					a.logger.Log(log.LevelDebug, "unsupported signal: "+sig.String())
					continue
				}

			}
		}

	}()
	return stateC
}

// Run is a no-op for Windows SCM since serviceMain is registered/invoked
// explicitly by the SCM outside of our control.
// serviceMain will do the work that Run would have been doing otherwise.
func (a *winscmAgent) Run(ctx context.Context) error {
	a.running.Store(true)

	a.logger.Log(log.LevelDebug, "starting winscm agent...")

	// retrieve a "service main" function
	serviceMain := a.serviceMainHandler()

	// Attempt to register a control handler callback.
	handlerPtr := syscall.NewCallback(serviceMain)
	if handlerPtr == 0 {
		return errors.New("failed to create callback function")
	}

	// the table slice must be null terminated, SCM looks for a null entry to know when to stop.
	// this entry is the service name and callback to "service main"
	entries := []serviceTableEntry{
		{serviceName: a.serviceName, serviceProc: handlerPtr},
		{serviceName: nil, serviceProc: 0},
	}

	select {
	case <-ctx.Done():
		return nil
	case a.signalC <- SignalStarting:
		// SignalStarting is not a signal from Windows SCM, it is a signal from the agent.
		a.logger.Log(log.LevelDebug, "sent starting signal")
	}

	// Start the service control dispatcher (blocking until serviceMain exits)
	ret, _, err := procStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&entries[0])))
	if ret == 0 || err != nil {
		if err != nil {
			return errors.New("Failed to start service control dispatcher: " + err.Error())
		}
		return err
	}

	return nil
}

// Notify sends a NotifyState to the agent's notifyC channel.
// Relay: WaitForSignals() loop -> a.notifyC -> serviceMain
func (a *winscmAgent) Notify(state NotifyState) error {
	if !a.running.Load() {
		return errors.New("cannot notify, agent is not running")
	}
	a.notifyC <- state
	return nil
}

func (a *winscmAgent) Close() error {
	if !a.running.Load() {
		return errors.New("cannot close, agent is not running")
	}

	if a.signalC != nil {
		close(a.signalC)
	}

	if a.notifyC != nil {
		close(a.notifyC)
	}

	return nil
}

// serviceMainHandler is a wrapper that returns the correct function signature for the Windows API.
// The returned function will be invoked by Windows SCM when it starts the service,
// it handles registering the service control handler with Windows SCM to relay control codes through.
func (a *winscmAgent) serviceMainHandler() func(argc uint32, argv **uint16) uintptr {
	return func(argc uint32, argv **uint16) uintptr {
		a.logger.Log(log.LevelDebug, "serviceMain invoked by Windows SCM")

		controlHandler := a.serviceCtrlHandler()

		// Register service control handler
		handle, _, err := procRegisterServiceCtrlHandlerExW.Call(
			uintptr(unsafe.Pointer(a.serviceName)),
			syscall.NewCallback(controlHandler),
			0, 0,
		)

		if handle == 0 && err != nil {
			// tell the daemon to stop, we can't continue without a control handler.
			a.logger.Log(log.LevelError, "failed to create callback function: "+err.Error())
			return 1
		}

		// store the handle for later use
		serviceStatusHandle := syscall.Handle(handle)

		lastStatus := serviceStatus{
			dwServiceType:             SERVICE_WIN32_OWN_PROCESS,
			dwCurrentState:            SERVICE_STOPPED,
			dwControlsAccepted:        0, // NOTE: Windows doesn't allow controls until service is running.
			dwWin32ExitCode:           0,
			dwServiceSpecificExitCode: 0,
			dwCheckPoint:              0,
			dwWaitHint:                0, // give SCM 5 seconds to start before timing out
		}

		// block until eventsC is closed
		// a.notifyC receives from daemon.go relayed from calling side of WaitForSignals.
		// serviceCtrlHandler -> a.signalC -> WaitForSignals
		// WaitForSignals() loop calls Notify() -> a.notifyC -> serviceMain
		for state := range a.notifyC {
			a.logger.Log(log.LevelDebug, "received notify state: "+state.String())
			switch state {
			case NotifyStarting:
				// should be the first message received.
				lastStatus := serviceStatus{
					dwServiceType:             SERVICE_WIN32_OWN_PROCESS,
					dwCurrentState:            SERVICE_START_PENDING,
					dwControlsAccepted:        0, // NOTE: Windows doesn't allow controls until service is running.
					dwWin32ExitCode:           0,
					dwServiceSpecificExitCode: 0,
					dwCheckPoint:              0,
					dwWaitHint:                2000, // give SCM 5 seconds to start before timing out
				}

				// notify the SCM that the service is starting
				err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
				if err != nil {
					// tell the daemon to stop, we can't continue without a control handler.
					a.logger.Log(log.LevelError, "Failed to set service status during START_PENDING: "+err.Error())
				}
				// notify starting isnt signaled by Windows SCM through service control handler.
				// its something that is sent during daemon Start so we need to ensure its passed
				// into WaitForSignals, so it can relay back Running notify.
				a.signalC <- SignalStarting

			case NotifyIsAlive:
				// just report the last status we seen.
				err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
				if err != nil {
					a.logger.Log(log.LevelError, "Failed to set service status on INTERROGATE: "+err.Error())
				}

			case NotifyRunning:
				lastStatus = serviceStatus{
					dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
					dwCurrentState:     SERVICE_RUNNING,
					dwControlsAccepted: SERVICE_ACCEPT_PARAMCHANGE | SERVICE_ACCEPT_STOP | SERVICE_ACCEPT_SHUTDOWN,
					dwWin32ExitCode:    0,
					dwCheckPoint:       0,
					dwWaitHint:         0,
				}
				// notify the SCM that the service is running
				err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
				if err != nil {
					a.logger.Log(log.LevelError, "Failed to set service status on RUNNING: "+err.Error())
				}

			case NotifyReloaded:
				lastStatus = serviceStatus{
					dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
					dwCurrentState:     SERVICE_RUNNING,
					dwControlsAccepted: SERVICE_ACCEPT_PARAMCHANGE | SERVICE_ACCEPT_STOP | SERVICE_ACCEPT_SHUTDOWN,
					dwWin32ExitCode:    0,
					dwCheckPoint:       0,
					dwWaitHint:         0,
				}

				// notify the SCM that the service is reloading
				err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
				if err != nil {
					a.logger.Log(log.LevelError, "Failed to set service status ON PARAMSCHANGE: "+err.Error())
				}

			case NotifyStopping:
				lastStatus = serviceStatus{
					dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
					dwCurrentState:     SERVICE_STOP_PENDING,
					dwControlsAccepted: 0,
					dwWin32ExitCode:    0,
					dwCheckPoint:       1,
					dwWaitHint:         5000,
				}

				// notify the SCM that the service is stopping
				err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
				if err != nil {
					a.logger.Log(log.LevelError, "Failed to set service status on STOPPING: "+err.Error())
				}

			case NotifyStopped:
				// this should be the last thing we see before a.notifyC is closed
				lastStatus = serviceStatus{
					dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
					dwCurrentState:     SERVICE_STOPPED,
					dwControlsAccepted: 0,
					dwWin32ExitCode:    0,
					dwCheckPoint:       0,
					dwWaitHint:         0,
				}

			default:
				// Handle unknown control codes.
				a.logger.Log(log.LevelError, "unexpected notify state: "+state.String())
			}
		}

		a.logger.Log(log.LevelDebug, "waiting for final notify state...")

		state := <-a.notifyC // if closed, immediate 0 value.
		if state != NotifyStopped {
			a.logger.Log(log.LevelError, "unexpected notify state: "+state.String())
		}

		lastStatus = serviceStatus{
			dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
			dwCurrentState:     SERVICE_STOPPED,
			dwControlsAccepted: 0,
			dwWin32ExitCode:    0,
			dwCheckPoint:       0,
			dwWaitHint:         0,
		}

		a.running.Store(false)
		// notify the SCM that the service has stopped
		err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
		if err != nil {
			// tell the daemon to stop, we can't continue without a control handler.
			a.logger.Log(log.LevelError, "Failed to set service status: "+err.Error())
			return 1
		}

		a.logger.Log(log.LevelDebug, "serviceMain completed")
		return 0
	}
}

// serviceCtrlHandler is a wrapper that return the appropriate function signature for the Windows API.
// This function must relay the control code as a signal to WaitForSignals.
// Relay: serviceCtrlHandler -> a.signalC -> WaitForSignals
func (a *winscmAgent) serviceCtrlHandler() func(control uint32, eventType uint32, eventData, context uintptr) uintptr {
	return func(control uint32, eventType uint32, eventData, context uintptr) uintptr {
		a.logger.Log(log.LevelDebug, "received control code: "+strconv.Itoa(int(control)))

		switch control {
		case SERVICE_CONTROL_STOP:
			// Relay a stop event to our agent's channel.
			a.signalC <- SignalStopping
			// Add additional cases for pause, continue, custom codes, etc.
		case SERVICE_CONTROL_SHUTDOWN:
			// Relay a shutdown event to our agent's channel.
			a.signalC <- SignalStopping
		case SERVICE_CONTROL_PAUSE:
			// IGNORE
		case SERVICE_CONTROL_CONTINUE:
			// IGNORE
		case SERVICE_CONTROL_INTERROGATE:
			// Relay an interrogate event to our agent's channel.
			a.signalC <- SignalAlive
		case SERVICE_CONTROL_PARAMCHANGE:
			// Relay a param change event to our agent's channel.
			a.signalC <- SignalReloading
		default:
			// Handle unknown control codes.
			return 1
		}
		// Return 0 to indicate successful processing.
		return 0
	}
}

// setServiceStatus updates the Windows SCM with the current service state,
// this function is called by the serviceMain function as it receives signals.
func (a *winscmAgent) setServiceStatus(handle syscall.Handle, status *serviceStatus) error {
	if status == nil {
		return errors.New("status cannot be nil")
	}

	ret, _, err := procSetServiceStatus.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(status)),
	)
	if ret == 0 && err != nil {
		return errors.New("failed to set service status: " + err.Error())
	}

	return nil
}
