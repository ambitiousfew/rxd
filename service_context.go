package rxd

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ambitiousfew/intracom"
	"golang.org/x/exp/slog"
)

// ServiceContext all services will require a config as a *ServiceContext in their service struct.
// This config contains preconfigured shutdown channel,
type ServiceContext struct {
	Name string
	Ctx  context.Context
	Log  *slog.Logger

	cancelCtx context.CancelFunc

	service Service
	opts    *serviceOpts

	iStates *intracom.Intracom[States]

	// ensure a service shutdown cannot be called twice
	stopCalled     atomic.Int32
	shutdownCalled atomic.Int32
	// mu is primarily used for mutations against isStopped and isShutdown between manager and wrapped service logic
	mu sync.Mutex
}

// NewService creates a new service context instance given a name and options.
func NewService(name string, service Service, opts *serviceOpts) *ServiceContext {
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &ServiceContext{
		Ctx:       ctx,
		cancelCtx: cancel,
		Name:      name,
		opts:      opts,

		iStates: intracom.New[States](),

		// 0 = not called, 1 = called
		shutdownCalled: atomic.Int32{},
		service:        service,
		Log:            opts.logger,

		mu: sync.Mutex{},
	}
}

// ShutdownSignal returns the channel the side implementing the service should use and watch to be notified
// when the daemon/manager are attempting to shutdown services.
func (sc *ServiceContext) ShutdownSignal() <-chan struct{} {
	return sc.Ctx.Done()
}

func (sc *ServiceContext) hasStopped() bool {
	return sc.stopCalled.Load() == 1
}

func (sc *ServiceContext) hasShutdown() bool {
	// 0 has not shutdown, 1 has shutdown
	return sc.shutdownCalled.Load() == 1
}

func (sc *ServiceContext) shutdown() {
	if sc.shutdownCalled.Swap(1) == 1 {
		// if we swap and the old value is already 1, shutdown was already called.
		return
	}
	sc.Log.Debug("sending a shutdown signal", "service", sc.Name)
	sc.cancelCtx()
}
