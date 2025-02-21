package rxd

import "context"

type DaemonAgent interface {
	Run(ctx context.Context) error
	Notify(state NotifyState) error
	Close() error
}

type noopAgent struct{}

func (n noopAgent) Run(_ context.Context) error {
	return nil
}

func (n noopAgent) Close() error {
	return nil
}

func (n noopAgent) Notify(_ NotifyState) error {
	return nil
}
