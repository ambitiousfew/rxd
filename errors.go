package rxd

const (
	ErrDaemonStarted            Error = Error("daemon has already been started")
	ErrDuplicateServiceName     Error = Error("duplicate service name found")
	ErrNoServices               Error = Error("no services to run")
	ErrNoServiceName            Error = Error("no service name provided")
	ErrNilService               Error = Error("nil service provided")
	ErrDuplicateServicePolicy   Error = Error("duplicate service policy found")
	ErrAddingServiceOnceStarted Error = Error("cannot add a service once the daemon is started")
)

type Error string

func (e Error) Error() string {
	return string(e)
}

type ErrUninitialized struct {
	StructName string
	Method     string
}

func (e ErrUninitialized) Error() string {
	return e.StructName + " is nil, but uses a value receiver for '" + e.Method + "' method."
}
