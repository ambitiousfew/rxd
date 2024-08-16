package rxd

import (
	"context"
	"time"

	"github.com/ambitiousfew/rxd/log"
)

type PrestartPipeline interface {
	Add(stage Stage)
	Run(ctx context.Context) <-chan DaemonLog
}

type StageFunc func(ctx context.Context) error

type Stage struct {
	Name string
	Func StageFunc
}

type prestartPipeline struct {
	RestartOnError bool          // If true, the pipeline will restart from the beginning if an error occurs
	RestartDelay   time.Duration // Delay between restarts
	Stages         []Stage       // Stages to run in order
}

type PrestartConfig struct {
	RestartOnError bool
	RestartDelay   time.Duration
}

func NewPrestartPipeline(conf PrestartConfig, stages ...Stage) PrestartPipeline {
	return &prestartPipeline{
		RestartOnError: conf.RestartOnError,
		RestartDelay:   conf.RestartDelay,
		Stages:         stages,
	}
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
