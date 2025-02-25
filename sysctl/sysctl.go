package sysctl

import "context"

type Agent interface {
	Run(ctx context.Context) error
	Notify(state NotifyState) error
	Close() error
}

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

func NewDefaultSystemAgent() Agent {
	return noopSystemAgent{}
}

// noopSystemAgent is a no-op system agent that will be used
// when no system agent is provided to the daemon.
// System agents are meant to be used when running the binary
// under a system service manager such as systemd, launchd, or Windows SCM.
type noopSystemAgent struct{}

func (n noopSystemAgent) Run(_ context.Context) error {
	return nil
}

func (n noopSystemAgent) Close() error {
	return nil
}

func (n noopSystemAgent) Notify(_ NotifyState) error {
	return nil
}
