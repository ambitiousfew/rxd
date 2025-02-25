//go:build windows

package sysctl

import (
	"context"
	"errors"
	"syscall"
	"unsafe"
)

// Windows API DLL and function references.
var (
	modAdvapi32                       = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcherW   = modAdvapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandlerExW = modAdvapi32.NewProc("RegisterServiceCtrlHandlerExW")
	procSetServiceStatus              = modAdvapi32.NewProc("SetServiceStatus")
)

// Constants for service states and control codes.
const (
	SERVICE_STOPPED       = 0x00000001
	SERVICE_START_PENDING = 0x00000002
	SERVICE_STOP_PENDING  = 0x00000003
	SERVICE_RUNNING       = 0x00000004

	SERVICE_CONTROL_STOP = 0x00000001
	// Add more control codes as needed.
)

// serviceStatus is our own version of the SERVICE_STATUS structure.
type serviceStatus struct {
	dwServiceType             uint32
	dwCurrentState            uint32
	dwControlsAccepted        uint32
	dwWin32ExitCode           uint32
	dwServiceSpecificExitCode uint32
	dwCheckPoint              uint32
	dwWaitHint                uint32
}

// serviceTableEntry mirrors the Windows SERVICE_TABLE_ENTRY structure.
type serviceTableEntry struct {
	serviceName *uint16
	serviceProc uintptr
}

type WindowsSCMAgentOption func(*winscmAgent)

func NewWindowsSCMAgent(serviceName string, opt ...WindowsSCMAgentOption) (*winscmAgent, error) {
	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return nil, err
	}

	agent := &winscmAgent{
		serviceName: serviceNamePtr,
		events:      make(chan uint32, 1),
	}

	for _, o := range opt {
		o(agent)
	}

	return agent, nil
}

type winscmAgent struct {
	serviceName *uint16
	events      chan uint32
}

// Run will act as our "service main" for Windows SCM
func (a *winscmAgent) Run(ctx context.Context) error {
	err := a.reportStatus(SERVICE_RUNNING, 0, 0)
	if err != nil {
		return err
	}

	// Register a control handler callback.
	handlerPtr := syscall.NewCallback(a.serviceCtrlHandler)
	if handlerPtr == 0 {
		return errors.New("Failed to create callback function.")
	}

	ret, _, err := procRegisterServiceCtrlHandlerExW.Call(
		uintptr(unsafe.Pointer(a.serviceName)),
		handlerPtr,
		0,
	)

	if ret == 0 || err != nil {
		return errors.New("Failed to register service control handler.")
	}

	<-ctx.Done()
	return nil
}

// serviceCtrlHandler is the callback that Windows SCM calls when a control code is sent.
func (a *winscmAgent) serviceCtrlHandler(control, eventType, eventData, context uintptr) uintptr {
	switch control {
	case SERVICE_CONTROL_STOP:
		// Relay a stop event to our agent's channel.
		a.events <- SERVICE_CONTROL_STOP
		// Add additional cases for pause, continue, custom codes, etc.
	default:
		// Handle unknown control codes.
		return 1
	}

	// Return 0 to indicate successful processing.
	return 0
}

// reportStatus calls SetServiceStatus to notify SCM of our current state.
func (a *winscmAgent) reportStatus(state, exitCode, waitHint uint32) error {
	var status serviceStatus
	status.dwServiceType = 0x00000010 // SERVICE_WIN32_OWN_PROCESS
	status.dwCurrentState = state
	// For simplicity, we’re only accepting stop commands.
	if state == SERVICE_RUNNING {
		status.dwControlsAccepted = 0x00000001 // Accepts SERVICE_CONTROL_STOP
	}
	status.dwWin32ExitCode = exitCode
	status.dwServiceSpecificExitCode = 0
	status.dwCheckPoint = 0
	status.dwWaitHint = waitHint

	ret, _, err := procSetServiceStatus.Call(
		uintptr(unsafe.Pointer(a.serviceName)),
		uintptr(unsafe.Pointer(&status)),
	)

	// how do i handle this error?
	if ret == 0 || err != nil {
		return errors.New("Failed to set service status.")
	}
	return nil
}
