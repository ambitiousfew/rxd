//go:build linux

package rxd

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type SystemdAgentOption func(*systemdAgent)

func NewSystemdAgent(opt ...SystemdAgentOption) *systemdAgent {
	agent := &systemdAgent{
		notifyReady:      false,
		enableWatchdog:   false,
		watchdogInterval: 0,
		conn:             nil,
		lastState:        NotifyStateStopped,
		mu:               sync.RWMutex{},
	}

	for _, o := range opt {
		o(agent)
	}
	return agent
}

func WithNotifyReady(notifyReady bool) SystemdAgentOption {
	return func(a *systemdAgent) {
		a.notifyReady = notifyReady
	}
}

func WithWatchdog(interval time.Duration) SystemdAgentOption {
	return func(a *systemdAgent) {
		a.enableWatchdog = true
		a.watchdogInterval = interval
	}
}

type systemdAgent struct {
	notifyReady      bool
	enableWatchdog   bool
	watchdogInterval time.Duration
	conn             *net.UnixConn

	lastState NotifyState
	mu        sync.RWMutex
}

func (a *systemdAgent) Run(ctx context.Context) error {
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

	// notify systemd that the service is ready

	if a.notifyReady {
		log.Println("Notifying systemd that the service is ready")
		err := a.Notify(NotifyStateReady)
		if err != nil {
			log.Println("Failed to notify systemd that the service is ready:", err)
			return err
		} else {
			log.Println("Notified systemd that the service is ready")
		}
	}

	if a.enableWatchdog && a.watchdogInterval > 0 {
		log.Println("Enabling systemd watchdog with interval", a.watchdogInterval)

		ticker := time.NewTicker(a.watchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if a.lastState == NotifyStateStopped || a.lastState == NotifyStateStopping {
					continue // skip until the service is other than stopped
				}
				err := a.Notify(NotifyStateAlive)
				if err != nil {
					log.Println("Failed to notify systemd watchdog:", err)
				} else {
					log.Println("Notified systemd watchdog")
				}
			}
		}
	}

	log.Println("Systemd agent is running...")
	<-ctx.Done()

	return nil
}

func (a *systemdAgent) Close() error {
	if a.conn != nil {
		return a.conn.Close()
	}

	return nil
}

func (a *systemdAgent) Notify(state NotifyState) error {
	var payload []byte
	switch state {
	case NotifyStateReady:
		payload = []byte("READY=1")
	case NotifyStateStopping:
		payload = []byte("STOPPING=1")
	case NotifyStateReloading:
		payload = []byte("RELOADING=1")
	case NotifyStateAlive:
		payload = []byte("WATCHDOG=1")
	default:
		return errors.New("'" + string(state) + "' unsupported state for systemd notifier")
	}

	_, err := a.conn.Write(payload)
	return err
}
