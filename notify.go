package rxd

import "context"

const (
	NotifyStateStopped NotifyState = iota
	NotifyStateStopping
	NotifyStateRestarting
	NotifyStateReloading
	NotifyStateReady
	NotifyStateAlive
)

type NotifyState uint8

func (s NotifyState) String() string {
	switch s {
	case NotifyStateStopped:
		return "STOPPED"
	case NotifyStateStopping:
		return "STOPPING"
	case NotifyStateRestarting:
		return "RESTARTING"
	case NotifyStateReloading:
		return "RELOADING"
	case NotifyStateReady:
		return "READY"
	case NotifyStateAlive:
		return "ALIVE"
	default:
		return ""
	}
}

type SystemNotifier interface {
	Start(ctx context.Context, errC chan<- DaemonLog) error
	Notify(state NotifyState) error
	// NotifyReady() error
	// NotifyReloading() error
	// NotifyStopping() error
	// NotifyAlive() error
}
