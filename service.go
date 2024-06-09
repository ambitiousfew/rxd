package rxd

import (
	"context"
	"time"
)

const (
	StateInit State = iota
	StateIdle
	StateRun
	StateStop
	StateExit
)

type State int

func (s State) String() string {
	switch s {
	case StateInit:
		return "init"
	case StateIdle:
		return "idle"
	case StateRun:
		return "run"
	case StateStop:
		return "stop"
	case StateExit:
		return "exit"
	default:
		return "unknown"
	}
}

type ServiceRunner interface {
	Init(context.Context) error
	Idle(context.Context) error
	Run(context.Context) error
	Stop(context.Context) error
}

// DaemonService is passed to the policy handler to have its ServiceRunner handled there.
type DaemonService struct {
	Name   string
	Runner ServiceRunner
}

// Service is used as a data-transfer object to pass to the Daemon, where it becomes a DaemonService to the policy handler.
type Service struct {
	Name    string
	Runner  ServiceRunner
	Handler ServiceHandler
}

func NewService(name string, runner ServiceRunner, options ...ServiceOption) Service {
	defaultPolicy := RunPolicyConfig{
		Policy:       PolicyRunContinous,
		RestartDelay: 3 * time.Second,
	}

	s := Service{
		Name:   name,
		Runner: runner,
		// reasonable default if policy handler isnt set
		Handler: GetServiceHandler(defaultPolicy),
	}

	for _, option := range options {
		option(&s)
	}

	return s
}
