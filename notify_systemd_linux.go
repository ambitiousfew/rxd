//go:build linux

package rxd

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type systemdNotifier struct {
	watchdog uint64
	conn     *net.UnixConn
}

func NewSystemdNotifier(socketName string, durationSecs uint64) (SystemNotifier, error) {
	if socketName == "" {
		// no socket name, no-op notifier
		return &systemdNotifier{}, nil
	}

	conn, err := net.Dial("unixgram", socketName)
	if err != nil {
		return nil, err
	}

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, errors.New("connection is not a unix connection type")
	}

	return &systemdNotifier{
		conn:     unixConn,
		watchdog: durationSecs,
	}, nil
}

func (n *systemdNotifier) Notify(state NotifyState) error {
	if n.watchdog == 0 {
		// do nothing if watchdog is not set
		return nil
	}

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

	_, err := n.conn.Write(payload)
	return err
}

func (n *systemdNotifier) Start(ctx context.Context, logC chan<- DaemonLog) error {
	if n.watchdog == 0 {
		// do nothing if watchdog is not set
		return nil
	}

	go func() {
		ticker := time.NewTicker(time.Duration(n.watchdog) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := n.Notify(NotifyStateAlive)
				if err != nil {
					// TODO: log error via channel??
					logC <- DaemonLog{
						Level:   log.LevelError,
						Message: err.Error(),
						Name:    "internal:systemd-notifier",
					}
				}
			}
		}
	}()
	return nil
}
