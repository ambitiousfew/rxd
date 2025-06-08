package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

// Pipeline is an interface that defines a sequence of stages to be executed in order.
// Each stage is a function that takes a context and returns an error.
type Pipeline interface {
	Add(stage Stage)
	Run(ctx context.Context) <-chan DaemonLog
}

// StageFunc is a function type that represents a stage in the pipeline.
// It takes a context and returns an error if the stage fails.
type StageFunc func(ctx context.Context) error

// Stage represents a single stage in the pipeline.
// It has a name for identification and a function to execute.
type Stage struct {
	Name string
	Func StageFunc
}

// PrestartConfig holds the configuration for a prestart pipeline.
type PrestartConfig struct {
	RestartOnError bool
	RestartDelay   time.Duration
}

// NewPrestartPipeline creates a new prestart pipeline with the given configuration and stages.
// The pipeline will run the stages in order and can be configured to restart on error with a delay.
// If RestartOnError is true, the pipeline will restart from the beginning if an error occurs.
func NewPrestartPipeline(conf PrestartConfig, stages ...Stage) Pipeline {
	return &prestartPipeline{
		RestartOnError: conf.RestartOnError,
		RestartDelay:   conf.RestartDelay,
		Stages:         stages,
	}
}

type prestartPipeline struct {
	RestartOnError bool          // If true, the pipeline will restart from the beginning if an error occurs
	RestartDelay   time.Duration // Delay between restarts
	Stages         []Stage       // Stages to run in order
}

func (p *prestartPipeline) Add(stage Stage) {
	p.Stages = append(p.Stages, stage)
}

func (p *prestartPipeline) Run(ctx context.Context) <-chan DaemonLog {
	errC := make(chan DaemonLog, 1)

	go func() {
		defer close(errC)

		if len(p.Stages) == 0 {
			return
		}

		if p.RestartDelay == 0 {
			p.RestartDelay = 5 * time.Second
		}

		timer := time.NewTimer(0)
		defer timer.Stop()

		var done bool
		for !done {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				// wait for the timer to expire before re-running the stages
			}

			var err error
			// run all preflight stages in order.
			for _, stage := range p.Stages {
				// before each stage run we check if the context is done
				select {
				case <-ctx.Done():
					return
				default:
				}

				err = stage.Func(ctx)
				if err != nil {
					lvl := log.LevelError
					if p.RestartOnError {
						lvl = log.LevelWarning
					}

					errC <- DaemonLog{
						Level:   log.Level(lvl),
						Message: "prestart failed",
						Fields:  []log.Field{log.Error("error", err), log.String("stage", stage.Name)},
					}

					if p.RestartOnError {
						timer.Reset(p.RestartDelay)
						break
					}
				}
			}

			if err == nil {
				// all stages completed successfully without error
				done = true
			}
		}
	}()

	return errC
}
