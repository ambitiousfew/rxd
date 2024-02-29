package rxd

type RxError string

func (e RxError) Error() string {
	return string(e)
}

const (
	ErrNoServices           RxError = RxError("no services")
	ErrServiceAlreadyExists RxError = RxError("service already exists")
)
