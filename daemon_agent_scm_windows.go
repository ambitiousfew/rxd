//go:build windows

package rxd

import (
	"context"
	"log"
)

type WindowsSCMAgentOption func(*winscmAgent)

func NewWindowsSCMAgent(opt ...WindowsSCMAgentOption) *winscmAgent {
	agent := &winscmAgent{}

	for _, o := range opt {
		o(agent)
	}
	return agent
}

func WithNotifyReady(thing string) WindowsSCMAgentOption {
	return func(a *winscmAgent) {
		a.thing = thing
	}
}

type winscmAgent struct {
	thing string
}

func (a *winscmAgent) Run(ctx context.Context) error {
	if a.thing == "" {
		// no-op
		return nil
	}

	log.Println("Windows SCM agent is running...", a.thing)
	<-ctx.Done()

	return nil
}
