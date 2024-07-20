//go:build linux

package rxd

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type systemdNotifier struct {
	watchdog uint64
	conn     *net.UnixConn
	mu       *sync.RWMutex
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
		mu:       &sync.RWMutex{},
	}, nil
}

func (n systemdNotifier) Notify(state NotifyState) error {
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

	n.mu.Lock()
	_, err := n.conn.Write(payload)
	n.mu.Unlock()
	return err
}

func (n systemdNotifier) Start(ctx context.Context, logger log.Logger) error {
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
					logger.Log(log.LevelError, "internal:systemd-notifier", log.Error("error", err))
				}
			}
		}
	}()
	return nil
}
