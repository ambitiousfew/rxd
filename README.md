# RxDaemon
A simple (alpha) reactive services daemon

## Example Service Template

```go
package main

import (
	"github.com/ambitiousfew/rxd"
)

type YourService struct {
	cfg *rxd.ServiceConfig
}


func (s *YourService) Name() string {
	return "YourServiceName"
}

func (s *YourService) Config() *rxd.ServiceConfig {
	return s.cfg
}

// Init can be used for pre-checks, loading configs, envs, etc
func (s *YourService) Init() rxd.ServiceResponse {
	// if all is well here, move to the next state or skip to RunState
	return rxd.NewResponse(nil, rxd.IdleState)
}

// Idle can be used for service status checks or used to fallback to a idle/retry state.
func (s *YourService) Idle() rxd.ServiceResponse {
	// if all is well here, move to the RunState or retry back to Init if something went wrong.
	return rxd.NewResponse(nil, rxd.RunState)
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *YourService) Run() rxd.ServiceResponse {
  
  // For anything that can block for any amount of time (long running logic/service)
  // we want to use a pattern such as this so we can always watch for the ShutdownC signal
  // This tells our service the Daemon received a signal to stop, we want to gracefully shutdown.
  for {
    select {
      case <-s.cfg.ShutdownC:
        return rxd.NewResponse(nil, rxd.StopState)
      default:
        // Do the core of your stuff here or on a timer interval...

        // If your service fails at anything you can transition your states
        // return rxd.NewResponse(nil, rxd.InitState)
        // return rxd.NewResponse(nil, rxd.IdleState) 
        // return rxd.NewResponse(nil, rxd.RunState) [start from the top of Run again]
        // return rxd.NewResponse(nil, rxd.StopState)
        // return rxd.NewResponse(nil, rxd.ExitState)
    }
  }
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *YourService) Stop() rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.ExitState)
}

// This line is purely for error checking to ensure we are meeting the Service interface.
var _ rxd.Service = &YourService{}
```

### Example Daemon Entrypoint
```go
// Example entrypoint
func main() {
	
  // We create an instance of our ServiceConfig
	yourSvcCfg := rxd.NewServiceConfig(
    // set a run policy, RunUntilStoppedPolicy is default
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
	)

	// We create an instance of our service
	yourSvc := NewHelloWorldService(yourSvcCfg)

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(yourSvc)

	// We can set the log severity we want to observe, LevelInfo is default
	daemon.SetLogSeverity(rxd.LevelAll)

	err := daemon.Start() // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
```