//go:build darwin

package rxd

import (
	"context"
	"log"
)

type LaunchdOption func(*launchdAgent)

func NewLaunchdAgent(opt ...LaunchdOption) *launchdAgent {
	agent := &launchdAgent{
		writePIDFile: false,
		pidFilePath:  "",
		extraEnv:     make(map[string]string),
	}

	for _, o := range opt {
		o(agent)
	}
	return agent
}

func WithWritePIDFile(pidFilePath string) LaunchdOption {
	return func(a *launchdAgent) {
		a.writePIDFile = true
		a.pidFilePath = pidFilePath
	}
}

func WithExtraEnv(env map[string]string) LaunchdOption {
	return func(a *launchdAgent) {
		for k, v := range env {
			a.extraEnv[k] = v
		}
	}
}

type launchdAgent struct {
	writePIDFile bool
	pidFilePath  string
	extraEnv     map[string]string
}

func (a *launchdAgent) Run(ctx context.Context) error {
	if a.writePIDFile {
		log.Println("Writing PID file to", a.pidFilePath)
		// no-op
		return nil
	}

	if len(a.extraEnv) > 0 {
		log.Println("Extra environment variables:")
		for k, v := range a.extraEnv {
			log.Printf("  %s=%s\n", k, v)
		}
	}

	log.Println("Launchd agent is running...")
	<-ctx.Done()

	return nil
}
