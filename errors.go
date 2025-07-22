package rxd

import (
	"context"
	"errors"
)

const (
	// ErrLifecycleDone indicates that the lifecycle has completed.
	ErrLifecycleDone Error = Error("lifecycle done")
	// ErrDaemonStarted indicates that the daemon has already been started.
	ErrDaemonStarted Error = Error("daemon has already been started")
	// ErrDuplicateServiceName indicates that a service with the same name already exists.
	ErrDuplicateServiceName Error = Error("duplicate service name found")
	// ErrNoServices indicates that there are no services to run.
	ErrNoServices Error = Error("no services to run")
	// ErrNoServiceName indicates that no service name was provided.
	ErrNoServiceName Error = Error("no service name provided")
	// ErrNilService indicates that a nil service was provided.
	ErrNilService Error = Error("nil service provided")
	// ErrDuplicateServicePolicy indicates that a service policy with the same name already exists.
	ErrDuplicateServicePolicy Error = Error("duplicate service policy found")
	// ErrAddingServiceOnceStarted indicates that a service cannot be added after the daemon has started.
	ErrAddingServiceOnceStarted Error = Error("cannot add a service once the daemon is started")
)

// Error is a simple string type that implements the error interface.
type Error string

// Error implements the error interface for the Error type.
func (e Error) Error() string {
	return string(e)
}

// ErrUninitialized is an error type that indicates a struct is uninitialized
type ErrUninitialized struct {
	StructName string
	Method     string
}

func (e ErrUninitialized) Error() string {
	return e.StructName + " is nil, but uses a value receiver for '" + e.Method + "' method."
}

// IsCancelled checks if the error is a context cancellation error.
func IsCancelled(err error) bool {
	return err == ErrLifecycleDone || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
