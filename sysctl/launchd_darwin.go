//go:build darwin

package sysctl

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/ambitiousfew/rxd/log"
)

var _ Agent = (*launchdAgent)(nil)

func NewLaunchdAgent(opt ...LaunchdOption) *launchdAgent {
	agent := &launchdAgent{
		notifyC: make(chan NotifyState, 1),
		signals: []os.Signal{
			os.Interrupt,
			syscall.SIGTERM,
			syscall.SIGHUP,
		},
		logger:  noopLogger{},
		running: atomic.Bool{},
	}

	for _, o := range opt {
		o(agent)
	}

	return agent
}

type launchdAgent struct {
	signals []os.Signal
	notifyC chan NotifyState
	logger  log.Logger
	running atomic.Bool
}

func (a *launchdAgent) WatchForSignals(ctx context.Context) <-chan SignalState {
	stateC := make(chan SignalState, 1)

	go func() {
		defer close(stateC)

		signalC := make(chan os.Signal, 1)
		signal.Notify(signalC, a.signals...)
		defer signal.Stop(signalC)

		select {
		case <-ctx.Done():
			return // exit
		case stateC <- SignalStarting:
			// inform caller we are starting the watcher
			// So it can relay back to launchd that services are running.
			a.logger.Log(log.LevelInfo, "launchd signal watcher is starting")
		}

		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-signalC:
				a.logger.Log(log.LevelDebug, "received signal: "+sig.String())
				switch sig {
				case os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT:
					stateC <- SignalStopping
				case syscall.SIGHUP:
					stateC <- SignalReloading
				default:
					a.logger.Log(log.LevelDebug, "unsupported signal: "+sig.String())
					continue
				}
			}
		}
	}()

	return stateC
}

func (a *launchdAgent) Run(ctx context.Context) error {
	a.running.Store(true)
	defer a.running.Store(false)

	a.logger.Log(log.LevelInfo, "launchd agent is running")

	// receive all notify signals until closed.
	for signal := range a.notifyC {
		a.logger.Log(log.LevelDebug, "received notify state: "+signal.String())
	}

	return nil
}

func (a *launchdAgent) Close() error {
	if !a.running.Load() {
		return errors.New("launchd agent is not running")
	}

	if a.notifyC != nil {
		close(a.notifyC)
	}

	return nil
}

func (a *launchdAgent) Notify(state NotifyState) error {
	if !a.running.Load() {
		return errors.New("launchd agent is not running")
	}

	a.notifyC <- state
	return nil
}

func (a *launchdAgent) SetLogger(logger log.Logger) {
	a.logger = logger
}
