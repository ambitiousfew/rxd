package rxd

import (
	"context"
	"log/slog"
)

type ServiceConfig struct {
	Name string
	// TODO: this becomes a RunPolicy instead with reasonable defaults.
	RunOpts []ServiceOption
	Log     *slog.Logger

	opts *serviceOpts
}

type ConfigurableService interface {
	Config() ServiceConfig
	Init(ctx context.Context) ServiceResponse
	Idle(ctx context.Context) ServiceResponse
	Run(ctx context.Context) ServiceResponse
	Stop(ctx context.Context) ServiceResponse
}
