//go:build windows

package sysctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/ambitiousfew/rxd/log"
)

var _ Agent = (*winscmAgent)(nil)

// Windows API DLL and function references.
var (
	modAdvapi32                       = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcherW   = modAdvapi32.NewProc("StartServiceCtrlDispatcherW")   // must be called first or service wont be recognized by SCM.
	procRegisterServiceCtrlHandlerExW = modAdvapi32.NewProc("RegisterServiceCtrlHandlerExW") // only works inside an already recognized running windows service.
	procSetServiceStatus              = modAdvapi32.NewProc("SetServiceStatus")              // used to respond with updates back to SCM.
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
	debugFile, err := os.Create(filepath.Join("C:\\", "rxd.log"))
	if err != nil {
		return nil, err
	}

	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		debugFile.Write([]byte("Failed to convert service name to UTF16: " + err.Error()))
		return nil, err
	}

	agent := &winscmAgent{
		serviceName: serviceNamePtr,
		signalC:     make(chan SignalState),
		notifyC:     make(chan NotifyState),
		debugFile:   debugFile,
		logger:      noopLogger{},
		running:     atomic.Bool{},
	}

	for _, o := range opt {
		o(agent)
	}

	// retrieve a "service main" function
	serviceMain := agent.serviceMainHandler()

	// Attempt to register a control handler callback.
	handlerPtr := syscall.NewCallback(serviceMain)
	if handlerPtr == 0 {
		debugFile.Write([]byte("Failed to create callback function"))
		return nil, errors.New("failed to create callback function")
	}

	// the table slice must be null terminated, SCM looks for a null entry to know when to stop.
	// this entry is the service name and callback to "service main"
	entries := []serviceTableEntry{
		{serviceName: serviceNamePtr, serviceProc: handlerPtr},
		{serviceName: nil, serviceProc: 0},
	}

	// Register the service control handler.
	ret, _, err := procStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&entries[0])))
	if ret == 0 || err != nil {
		if err != nil {
			debugFile.Write([]byte("Failed to start service control dispatcher: " + err.Error()))
			return nil, errors.New("Failed to start service control dispatcher: " + err.Error())
		}

		debugFile.Write([]byte("Failed to start service control dispatcher"))
		return nil, err
	}

	return agent, nil
}

type winscmAgent struct {
	serviceName *uint16
	signalC     chan SignalState
	notifyC     chan NotifyState
	debugFile   *os.File
	logger      log.Logger
	running     atomic.Bool
}

func (a *winscmAgent) SetLogger(logger log.Logger) {
	a.logger = logger
}

func (a *winscmAgent) WatchForSignals(ctx context.Context) <-chan SignalState {
	stateC := make(chan SignalState)
	go func() {
		defer close(stateC)

		select {
		case <-ctx.Done():
			return // exit
		case stateC <- SignalStarting:
			// inform caller we are starting the watcher
			// So it can relay back to systemd that services are running.
			a.logger.Log(log.LevelInfo, "systemd signal watcher is starting")
		}

		for {
			select {
			case <-ctx.Done():
				return
			case sig, open := <-a.signalC:
				if !open {
					return
				}

				a.debugFile.WriteString("received signal: " + sig.String() + "\n")
				a.logger.Log(log.LevelDebug, "received signal: "+sig.String())

				switch sig {
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

	a.debugFile.WriteString("agent is running...\n")
	<-ctx.Done()
	a.debugFile.WriteString("agent is stopping...\n")
	// TODO: Could set any atomic state...then
	// TODO: I could hold this open with a done channel
	return nil
}

// Notify sends a NotifyState to the agent's notifyC channel.
// Relay: WaitForSignals() loop -> a.notifyC -> serviceMain
func (a *winscmAgent) Notify(state NotifyState) error {
	if !a.running.Load() {
		return errors.New("cannot notify, agent is not running")
	}
	a.debugFile.WriteString("sending notify: " + state.String() + "\n")
	a.notifyC <- state
	return nil
}

func (a *winscmAgent) Close() error {
	if !a.running.Load() {
		return errors.New("cannot close, agent is not running")
	}

	a.debugFile.WriteString("closing agent...\n")

	if a.signalC != nil {
		close(a.signalC)
	}

	if a.notifyC != nil {
		close(a.notifyC)
	}

	if a.debugFile != nil {
		a.debugFile.Close()
	}

	return nil
}

// serviceMainHandler is a wrapper that returns the correct function signature for the Windows API.
// The returned function will be invoked by Windows SCM when it starts the service,
// it handles registering the service control handler with Windows SCM to relay control codes through.
func (a *winscmAgent) serviceMainHandler() func(argc uint32, argv **uint16) uintptr {
	return func(argc uint32, argv **uint16) uintptr {
		controlHandler := a.serviceCtrlHandler()

		// Register service control handler
		handle, _, err := procRegisterServiceCtrlHandlerExW.Call(
			uintptr(unsafe.Pointer(a.serviceName)),
			syscall.NewCallback(controlHandler),
			0, 0,
		)

		if handle == 0 && err != nil {
			// tell the daemon to stop, we can't continue without a control handler.
			a.debugFile.WriteString("Failed to create callback function: " + err.Error() + "\n")
			return 1
		}

		// store the handle for later use
		serviceStatusHandle := syscall.Handle(handle)

		lastStatus := serviceStatus{
			dwServiceType:             SERVICE_WIN32_OWN_PROCESS,
			dwCurrentState:            SERVICE_START_PENDING,
			dwControlsAccepted:        0, // NOTE: Windows doesn't allow controls until service is running.
			dwWin32ExitCode:           0,
			dwServiceSpecificExitCode: 0,
			dwCheckPoint:              1,
			dwWaitHint:                5000, // give SCM 5 seconds to start before timing out
		}

		// notify the SCM that the service is starting
		err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
		if err != nil {
			// tell the daemon to stop, we can't continue without a control handler.
			a.debugFile.WriteString("Failed to set service status: " + err.Error() + "\n")
			return 1
		}

		// block until eventsC is closed
		// a.notifyC receives from daemon.go relayed from calling side of WaitForSignals.

		// serviceCtrlHandler -> a.signalC -> WaitForSignals
		// WaitForSignals() loop calls Notify() -> a.notifyC -> serviceMain
		for state := range a.notifyC {
			a.debugFile.WriteString("received notify state: " + state.String() + "\n")
			switch state {
			case NotifyIsAlive:
				// just report the last status we seen.
				err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
				if err != nil {
					// tell the daemon to stop, we can't continue without a control handler.
					// log it continue?
					a.debugFile.WriteString("Failed to set service status on INTERROGATE: " + err.Error() + "\n")
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
					// tell the daemon to stop, we can't continue without a control handler.
					// log it continue?
					a.debugFile.WriteString("Failed to set service status on RUNNING: " + err.Error() + "\n")
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
					// tell the daemon to stop, we can't continue without a control handler.
					// log it continue?
					a.debugFile.WriteString("Failed to set service status ON PARAMSCHANGE: " + err.Error() + "\n")
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
					// tell the daemon to stop, we can't continue without a control handler.
					// log it continue?
					a.debugFile.WriteString("Failed to set service status on STOPPING: " + err.Error() + "\n")
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
				a.debugFile.WriteString("unexpected notify state: " + state.String() + "\n")
			}
		}

		state := <-a.notifyC // if closed, immediate 0 value.
		if state != NotifyStopped {
			a.logger.Log(log.LevelError, "unexpected notify state: "+state.String())
			// return // exit
			a.debugFile.WriteString("unexpected notify state: " + state.String() + "\n")
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
			a.debugFile.WriteString("Failed to set service status: " + err.Error() + "\n")
			return 1
		}
		return 0
	}
}

// func (a *winscmAgent) serviceMain(argc uint32, argv **uint16) uintptr {
// 	controlHandler := a.serviceCtrlHandler()

// 	// Register service control handler
// 	// err is always non-nil due to the Call behavior.
// 	// if handle is not valid (0) then pay attention to err
// 	handle, _, err := procRegisterServiceCtrlHandlerExW.Call(
// 		uintptr(unsafe.Pointer(a.serviceName)),
// 		syscall.NewCallback(controlHandler),
// 		0, 0,
// 	)

// 	if handle == 0 && err != nil {
// 		a.debugFile.WriteString("Failed to create callback function: " + err.Error() + "\n")
// 		// tell the daemon to stop, we can't continue without a control handler.
// 		return 1
// 	}

// 	// store the handle for later use
// 	serviceStatusHandle := syscall.Handle(handle)

// 	lastStatus := serviceStatus{
// 		dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
// 		dwCurrentState:     SERVICE_START_PENDING,
// 		dwControlsAccepted: 0, // NOTE: Windows doesn't allow controls until service is running.
// 		dwWin32ExitCode:    0,
// 		dwCheckPoint:       1,
// 		dwWaitHint:         5000, // give SCM 5 seconds to start before timing out
// 	}

// 	// notify the SCM that the service is starting
// 	err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 	if err != nil {
// 		// tell the daemon to stop, we can't continue without a control handler.
// 		a.debugFile.WriteString("Failed to set service status during START_PENDING: " + err.Error() + "\n")
// 		// lastStatus.dwCurrentState = SERVICE_STOPPED
// 		// a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 		return 1
// 	}

// 	// block until eventsC is closed
// 	// a.notifyC receives from daemon.go relayed from calling side of WaitForSignals.

// 	// serviceCtrlHandler -> a.signalC -> WaitForSignals
// 	// WaitForSignals() loop calls Notify() -> a.notifyC -> serviceMain
// 	for state := range a.notifyC {
// 		switch state {
// 		case NotifyIsAlive:
// 			// just report the last status we seen.
// 			err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 			if err != nil {
// 				// tell the daemon to stop, we can't continue without a control handler.
// 				// log it continue?
// 				a.debugFile.WriteString("Failed to set service status on INTERROGATE: " + err.Error() + "\n")
// 			}

// 		case NotifyRunning:
// 			lastStatus = serviceStatus{
// 				dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
// 				dwCurrentState:     SERVICE_RUNNING,
// 				dwControlsAccepted: SERVICE_ACCEPT_PARAMCHANGE | SERVICE_ACCEPT_STOP | SERVICE_ACCEPT_SHUTDOWN,
// 				dwWin32ExitCode:    0,
// 				dwCheckPoint:       0,
// 				dwWaitHint:         0,
// 			}
// 			// notify the SCM that the service is running
// 			err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 			if err != nil {
// 				// tell the daemon to stop, we can't continue without a control handler.
// 				// log it continue?
// 				a.debugFile.WriteString("Failed to set service status on RUNNING: " + err.Error() + "\n")
// 			}

// 		case NotifyReloaded:
// 			lastStatus = serviceStatus{
// 				dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
// 				dwCurrentState:     SERVICE_RUNNING,
// 				dwControlsAccepted: SERVICE_ACCEPT_PARAMCHANGE | SERVICE_ACCEPT_STOP | SERVICE_ACCEPT_SHUTDOWN,
// 				dwWin32ExitCode:    0,
// 				dwCheckPoint:       0,
// 				dwWaitHint:         0,
// 			}

// 			// notify the SCM that the service is reloading
// 			err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 			if err != nil {
// 				// tell the daemon to stop, we can't continue without a control handler.
// 				// log it continue?
// 				a.debugFile.WriteString("Failed to set service status ON PARAMSCHANGE: " + err.Error() + "\n")
// 			}

// 		case NotifyStopping:
// 			lastStatus = serviceStatus{
// 				dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
// 				dwCurrentState:     SERVICE_STOP_PENDING,
// 				dwControlsAccepted: 0,
// 				dwWin32ExitCode:    0,
// 				dwCheckPoint:       1,
// 				dwWaitHint:         5000,
// 			}

// 			// notify the SCM that the service is stopping
// 			err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 			if err != nil {
// 				// tell the daemon to stop, we can't continue without a control handler.
// 				// log it continue?
// 				a.debugFile.WriteString("Failed to set service status on STOPPING: " + err.Error() + "\n")
// 			}

// 		case NotifyStopped:
// 			// this should be the last thing we see before a.notifyC is closed
// 			lastStatus = serviceStatus{
// 				dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
// 				dwCurrentState:     SERVICE_STOPPED,
// 				dwControlsAccepted: 0,
// 				dwWin32ExitCode:    0,
// 				dwCheckPoint:       0,
// 				dwWaitHint:         0,
// 			}

// 		default:
// 			// Handle unknown control codes.
// 			a.logger.Log(log.LevelError, "unexpected notify state on default: "+state.String())
// 			a.debugFile.WriteString("unexpected notify state: " + state.String() + "\n")
// 		}
// 	}

// 	state := <-a.notifyC // if closed, immediate 0 value.
// 	if state != NotifyStopped {
// 		a.logger.Log(log.LevelError, "unexpected notify state: "+state.String())
// 		a.debugFile.WriteString("unexpected notify state: " + state.String() + "\n")
// 		// return // exit
// 	}

// 	lastStatus = serviceStatus{
// 		dwServiceType:      SERVICE_WIN32_OWN_PROCESS,
// 		dwCurrentState:     SERVICE_STOPPED,
// 		dwControlsAccepted: 0,
// 		dwWin32ExitCode:    0,
// 		dwCheckPoint:       0,
// 		dwWaitHint:         0,
// 	}

// 	a.running.Store(false)
// 	// notify the SCM that the service has stopped
// 	err = a.setServiceStatus(serviceStatusHandle, &lastStatus)
// 	if err != nil {
// 		// tell the daemon to stop, we can't continue without a control handler.
// 		a.debugFile.WriteString("Failed to set service status: " + err.Error() + "\n")
// 	}

// 	return 0
// }

// serviceCtrlHandler is a wrapper that return the appropriate function signature for the Windows API.
// This function must relay the control code as a signal to WaitForSignals.
// Relay: serviceCtrlHandler -> a.signalC -> WaitForSignals
func (a *winscmAgent) serviceCtrlHandler() func(control uint32, eventType uint32, eventData, context uintptr) uintptr {
	return func(control uint32, eventType uint32, eventData, context uintptr) uintptr {
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

// func (a *winscmAgent) serviceCtrlHandler(control uint32, eventType uint32, eventData, context uintptr) uintptr {
// 	switch control {
// 	case SERVICE_CONTROL_STOP:
// 		// Relay a stop event to our agent's channel.
// 		// a.signalC <- SignalStopping
// 		// Add additional cases for pause, continue, custom codes, etc.
// 	case SERVICE_CONTROL_SHUTDOWN:
// 		// Relay a shutdown event to our agent's channel.
// 		// a.signalC <- SignalStopping
// 	case SERVICE_CONTROL_PAUSE:
// 		// IGNORE
// 	case SERVICE_CONTROL_CONTINUE:
// 		// IGNORE
// 	case SERVICE_CONTROL_INTERROGATE:
// 		// Relay an interrogate event to our agent's channel.
// 		// a.signalC <- SignalAlive
// 	case SERVICE_CONTROL_PARAMCHANGE:
// 		// Relay a param change event to our agent's channel.
// 		// a.signalC <- SignalReloading
// 	default:
// 		// Handle unknown control codes.
// 		return 1
// 	}
// 	// Return 0 to indicate successful processing.
// 	return 0
// }

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
