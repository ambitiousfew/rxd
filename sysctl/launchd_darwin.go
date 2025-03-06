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

type LaunchdOption func(*launchdAgent)

func NewLaunchdAgent(opt ...LaunchdOption) *launchdAgent {
	agent := &launchdAgent{
		writePIDFile: false,
		pidFilePath:  "",
		// extraEnv:     make(map[string]string),
		notifyC: make(chan NotifyState),
		signalC: make(chan SignalState),
		signals: []os.Signal{os.Interrupt, syscall.SIGINT, syscall.SIGTERM},
		logger:  noopLogger{},
		running: atomic.Bool{},
	}

	for _, o := range opt {
		o(agent)
	}
	return agent
}

func WithWritePIDFile(pidFilePath string) LaunchdOption {
	return func(a *launchdAgent) {
		a.writePIDFile = true
		a.pidFilePath = pidFilePath
	}
}

// func WithExtraEnv(env map[string]string) LaunchdOption {
// 	return func(a *launchdAgent) {
// 		for k, v := range env {
// 			a.extraEnv[k] = v
// 		}
// 	}
// }

type launchdAgent struct {
	writePIDFile bool
	pidFilePath  string
	signals      []os.Signal
	logger       log.Logger
	notifyC      chan NotifyState
	signalC      chan SignalState
	// extraEnv     map[string]string
	running atomic.Bool
}

func (a *launchdAgent) WatchForSignals(ctx context.Context) <-chan SignalState {
	stateC := make(chan SignalState, 1)

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
			case sig := <-a.signalC:
				a.logger.Log(log.LevelDebug, "received signal: "+sig.String())
				switch sig {
				case SignalStopping, SignalRestarting:
					stateC <- SignalStopping
				case SignalReloading:
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

	select {
	case <-ctx.Done():
		return nil
	case a.notifyC <- NotifyStarting:
		a.logger.Log(log.LevelDebug, "sent starting state")
	}

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, a.signals...)
	defer signal.Stop(signalC)

	for {
		select {
		case <-ctx.Done():
			return nil
		case state, open := <-a.notifyC:
			if !open {
				return nil
			}
			// receive all notify signals until closed.
			a.logger.Log(log.LevelDebug, "received notify state: "+state.String())
		case signal := <-signalC:
			switch signal {
			case os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT:
				a.signalC <- SignalStopping
			case syscall.SIGHUP:
				a.signalC <- SignalReloading
			default:
				a.logger.Log(log.LevelDebug, "unsupported signal: "+signal.String())
				continue

			}
		}
	}
}

func (a *launchdAgent) Close() error {
	if a.notifyC != nil {
		close(a.notifyC)
	}

	return nil
}

func (a *launchdAgent) Notify(state NotifyState) error {
	if !a.running.Load() {
		return errors.New("systemd agent is not running")
	}

	a.notifyC <- state
	return nil
}

func (a *launchdAgent) SetLogger(logger log.Logger) {
	a.logger = logger
}
