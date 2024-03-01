package rxd

type RxError string

func (e RxError) Error() string {
	return string(e)
}

const (
	ErrNoServices           RxError = RxError("no services")
	ErrServiceAlreadyExists RxError = RxError("service already exists")
	ErrAlreadyStarted       RxError = RxError("already started")
	ErrServiceNameRequired  RxError = RxError("service name required")
	ErrServiceSetupTimeout  RxError = RxError("service setup timeout")
)
