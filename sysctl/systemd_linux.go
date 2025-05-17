//go:build linux

package sysctl

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ambitiousfew/rxd/v2/log"
)

var _ Agent = (*systemdAgent)(nil)

func NewSystemdAgent(opt ...SystemdAgentOption) *systemdAgent {
	agent := &systemdAgent{
		notifyReady:      false,
		enableWatchdog:   false,
		watchdogInterval: 0,
		conn:             nil,
		notifyC:          make(chan NotifyState, 1),
		signals: []os.Signal{
			os.Interrupt,
			syscall.SIGTERM,
			syscall.SIGHUP,
		},
		mu:      sync.RWMutex{},
		running: atomic.Bool{},
	}

	for _, o := range opt {
		o(agent)
	}

	return agent
}

type systemdAgent struct {
	notifyReady      bool
	readyTimeout     time.Duration
	enableWatchdog   bool
	watchdogInterval time.Duration
	conn             *net.UnixConn
	signals          []os.Signal
	notifyC          chan NotifyState

	logger    log.Logger
	lastState NotifyState
	mu        sync.RWMutex
	running   atomic.Bool
}

func (a *systemdAgent) WatchForSignals(ctx context.Context) <-chan SignalState {
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
			// So it can relay back to systemd that services are running.
			a.logger.Log(log.LevelInfo, "systemd signal watcher is starting")
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

func (a *systemdAgent) Run(ctx context.Context) error {
	a.running.Store(true)
	defer a.running.Store(false)

	if !a.notifyReady && !a.enableWatchdog {
		// no-op
		return nil
	}

	conn, err := net.Dial("unixgram", os.Getenv("NOTIFY_SOCKET"))
	if err != nil {
		return err
	}

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return errors.New("connection is not a unix connection type")
	}

	a.conn = unixConn

	a.logger.Log(log.LevelInfo, "systemd agent is running")
	// readyNotify checks if notifyReady is enabled and sends the ready signal
	// to systemd. It will block until the signal is sent or the context is done.
	// If the readyTimeout is set and exceeded, it will timeout and return an error.
	// readyTimeout should be set to TimeoutStartSec in the systemd service file.
	err = a.readyNotify(ctx)
	if err != nil {
		return err
	}

	// only if watchdog is enabled and has interval (WatchdogSec is set)
	if a.enableWatchdog && a.watchdogInterval > 0 {
		err := a.handleWatchdog(ctx)
		if err != nil {
			return err
		}
	}

	// receive all notify signals until closed.
	for signal := range a.notifyC {
		a.logger.Log(log.LevelDebug, "received notify state: "+signal.String())
	}

	return nil
}

func (a *systemdAgent) Close() error {
	if a.conn != nil {
		a.conn.Close()
	}

	if a.notifyC != nil {
		close(a.notifyC)
	}

	return nil
}

func (a *systemdAgent) Notify(state NotifyState) error {
	if a.notifyReady && !a.running.Load() {
		return errors.New("systemd agent is not running")
	}

	a.notifyC <- state
	return nil
}

func (a *systemdAgent) SetLogger(logger log.Logger) {
	a.logger = logger
}

func (a *systemdAgent) notify(signal SignalState) error {
	var payload []byte
	switch signal {
	case SignalRunning:
		payload = []byte("READY=1")
	case SignalStopping:
		payload = []byte("STOPPING=1")
	case SignalReloading:
		payload = []byte("RELOADING=1")
	case SignalAlive:
		payload = []byte("WATCHDOG=1")
	// case SignalStatus: // not supported yet
	// 	payload = []byte("STATUS=" + status)
	default:
		return errors.New("'" + string(signal) + "' unsupported state for systemd notifier")
	}

	_, err := a.conn.Write(payload)
	return err
}

func (a *systemdAgent) readyNotify(ctx context.Context) error {
	if !a.notifyReady {
		return nil
	}

	a.logger.Log(log.LevelInfo, "systemd ready notify is enabled")

	timer := time.NewTimer(a.readyTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil
	case state, open := <-a.notifyC:
		if !open {
			return nil
		}

		switch state {
		case NotifyRunning:
			err := a.notify(SignalRunning)
			if err != nil {
				return err
			}
			a.logger.Log(log.LevelInfo, "notified systemd ready signal")
		default:
			return errors.New("unsupported state for systemd notifier")
		}
	}

	return nil
}

func (a *systemdAgent) handleWatchdog(ctx context.Context) error {
	a.logger.Log(log.LevelInfo, "enabling systemd watchdog with interval", log.Int("interval_ms", int(a.watchdogInterval.Milliseconds())))

	var lastState SignalState

	ticker := time.NewTicker(a.watchdogInterval)
	defer ticker.Stop()

watchdogloop:
	for {
		select {
		case <-ctx.Done():
			break watchdogloop

		case state, open := <-a.notifyC:
			// update the last state
			if !open {
				return nil
			}

			switch state {
			case NotifyStopped:
				lastState = SignalStopping
				return nil // exit??
			case NotifyStarted:
				lastState = SignalStarting
				// TODO: // if we leverage status maybe we could send this status?
			case NotifyReloaded:
				lastState = SignalRunning
				// TODO: // if we leverage status maybe we could send this status?
			case NotifyRunning:
				lastState = SignalRunning
			default:
				continue
			}

			err := a.notify(lastState)
			if err != nil {
				a.logger.Log(log.LevelError, "failed to notify systemd watchdog", log.Error("error", err))
			} else {
				a.logger.Log(log.LevelInfo, "notified systemd watchdog", log.String("state", string(lastState)))
			}

		case <-ticker.C:
			// if the watchdog ticker ticks, we send an alive signal
			if lastState == SignalRunning {
				err := a.notify(SignalAlive)
				if err != nil {
					return err
				}
				a.logger.Log(log.LevelDebug, "notified systemd watchdog", log.String("state", string(SignalAlive)))
			}

		}
	}
	// wait for notifyC to close, then exit
	for range a.notifyC {
	}
	a.logger.Log(log.LevelInfo, "systemd watchdog handler is closing")
	return nil
}
