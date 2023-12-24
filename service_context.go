package rxd

import (
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

// ServiceContext all services will require a config as a *ServiceContext in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceContext struct {
	Name string

	service Service
	opts    *serviceOpts

	intracom *intracom.Intracom[[]byte]

	Log *slog.Logger

	cancel chan struct{} // a channel all services should watch for shutdown signal

	// ensure a service shutdown cannot be called twice
	isStopped  atomic.Int32
	isShutdown atomic.Int32

	mu sync.Mutex
}

// ShutdownSignal returns the channel the side implementing the service should use and watch to be notified
// when the daemon/manager are attempting to shutdown services.
func (sc *ServiceContext) ShutdownSignal() <-chan struct{} {
	return sc.cancel
}

func (sc *ServiceContext) IntracomRegister(topic string) (chan<- []byte, func() bool) {
	return sc.intracom.Register(topic)
}

// IntracomSubscribe subscribes this service to inter-service communication by its topic name.
// A channel is returned to receive published messages from another service.
// RxD standardizes on slice of bytes since it allows us to pass messages using a built-in type and
// easily leverage json unmarshal into a custom struct if needed.
func (sc *ServiceContext) IntracomSubscribe(topic string, id string) (<-chan []byte, func() error) {
	return sc.intracom.Subscribe(&intracom.SubscriberConfig{
		Topic:         topic,
		ConsumerGroup: id,
		BufferSize:    1,
		BufferPolicy:  intracom.DropNone,
	})
}

func (sc *ServiceContext) hasStopped() bool {
	return sc.isStopped.Load() == 1
}

func (sc *ServiceContext) hasShutdown() bool {
	// 0 has not shutdown, 1 has shutdown
	return sc.isShutdown.Load() == 1
}

func (sc *ServiceContext) setLogger(logger *slog.Logger) {
	// sc.Log = logger
	sc.Log = logger.With("service", sc.Name)
}

func (sc *ServiceContext) setIntracom(intracom *intracom.Intracom[[]byte]) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.intracom = intracom
}

func (sc *ServiceContext) shutdown() <-chan struct{} {
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		// ensure shutdown only closes once
		if sc.isShutdown.Swap(1) == 1 {
			return
		}
		close(sc.cancel)
	}()

	return doneC
}
