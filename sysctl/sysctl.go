package sysctl

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/ambitiousfew/rxd/v2/log"
)

type Agent interface {
	Run(ctx context.Context) error
	Notify(state NotifyState) error
	WatchForSignals(ctx context.Context) <-chan SignalState
	SetLogger(logger log.Logger)
	Close() error
}

type SignalState uint8

const (
	SignalStopping SignalState = iota
	SignalRestarting
	SignalStarting
	SignalReloading
	SignalRunning
	SignalAlive
)

func (s SignalState) String() string {
	switch s {
	case SignalStopping:
		return "stopping"
	case SignalRestarting:
		return "restarting"
	case SignalStarting:
		return "starting"
	case SignalReloading:
		return "reloading"
	case SignalRunning:
		return "running"
	case SignalAlive:
		return "alive"
	default:
		return ""
	}
}

type NotifyState uint8

const (
	NotifyStopped NotifyState = iota
	NotifyStopping
	NotifyStarted
	NotifyStarting
	NotifyReloaded
	NotifyReloading
	NotifyRunning
	NotifyIsAlive
)

func (s NotifyState) String() string {
	switch s {
	case NotifyStopped:
		return "stopped"
	case NotifyStarted:
		return "started"
	case NotifyReloaded:
		return "reloaded"
	case NotifyRunning:
		return "running"
	case NotifyIsAlive:
		return "alive"
	default:
		return ""
	}
}

// NewDefaultSystemAgent returns a default system agent that does
// not run any platform-specific system control agent for the daemon.
// This is useful when running the daemon as a standalone process.
func NewDefaultSystemAgent(opts ...DefaultOption) Agent {
	agent := &defaultAgent{
		signals: []os.Signal{
			os.Interrupt,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGHUP,
		},
		logger:  noopLogger{},
		notifyC: make(chan NotifyState),
		running: atomic.Bool{},
	}

	for _, opt := range opts {
		opt(agent)
	}
	return agent
}

// defaultAgent is a no-op system agent that will be used
// when no system agent is provided to the daemon.
// System agents are meant to be used when running the binary
// under a system service manager such as systemd, launchd, or Windows SCM.
type defaultAgent struct {
	signals []os.Signal
	notifyC chan NotifyState
	running atomic.Bool
	logger  log.Logger
}

func (a *defaultAgent) Run(ctx context.Context) error {
	a.running.Swap(true)
	defer a.running.Swap(false)

	// block until notifyC is closed
	for state := range a.notifyC {
		a.logger.Log(log.LevelDebug, "received notify state: "+state.String())
	}

	return nil
}

func (a *defaultAgent) WatchForSignals(ctx context.Context) <-chan SignalState {
	ch := make(chan SignalState)
	go func() {
		defer close(ch)

		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, a.signals...)
		defer signal.Stop(signalC)

		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-signalC:
				a.logger.Log(log.LevelDebug, "received signal: "+sig.String())
				switch sig {
				case os.Interrupt, syscall.SIGTERM:
					ch <- SignalStopping
					return // exit
				case syscall.SIGHUP:
					ch <- SignalReloading
				default:
					continue
				}
			}
		}
	}()

	return ch
}

func (a *defaultAgent) SetLogger(logger log.Logger) {
	a.logger = logger
}

func (a *defaultAgent) Close() error {
	if a.notifyC != nil {
		close(a.notifyC)
		a.notifyC = nil
	}

	return nil
}

func (a *defaultAgent) Notify(state NotifyState) error {
	if !a.running.Load() {
		return errors.New("agent is not running")
	}

	if a.notifyC != nil {
		a.notifyC <- state
	}

	return nil
}
