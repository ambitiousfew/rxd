package main

import (
	"encoding/json"
	"fmt"
	"io"

	"net/http"
	"time"

	"github.com/ambitiousfew/rxd"
	"github.com/ambitiousfew/rxd/log"
)

// APIPollingService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type APIPollingService struct {
	// fields this specific server uses
	client        *http.Client
	apiBase       string
	retryDuration time.Duration
	maxPollCount  int
}

// NewAPIPollingService just a factory helper function to help create and return a new instance of the service.
func NewAPIPollingService() *APIPollingService {
	return &APIPollingService{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		// We will check every 10s to see if we can establish a connection to the API when Idle retrying.
		retryDuration: 10 * time.Second,
		apiBase:       "http://localhost:8000",
		maxPollCount:  5,
	}
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *APIPollingService) Idle(ctx rxd.ServiceContext) error {
	ctx.Log(log.LevelInfo, "entered idle state")
	statesC, cancel := ctx.WatchAllServices(rxd.WatchConfig{
		Action:      rxd.Entering,
		TargetState: rxd.StateRun,
	}, ServiceHelloWorldAPI)

	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-statesC:
			ctx.Log(log.LevelInfo, "Hello World API has entered Run state")
			// Hello World API should have entered a RunState, so we can move to our next state (Run) now.
			return nil
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *APIPollingService) Run(ctx rxd.ServiceContext) error {
	ctx.Log(log.LevelInfo, "entered run state")
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	// Here we are registering our interest in ANY of the services passed EXITING a "RunState"
	// So if any service given here for some reasons LEAVES their RunState, we will be notified.

	statesC, cancel := ctx.WatchAllServices(rxd.WatchConfig{
		Action:      rxd.Exiting,
		TargetState: rxd.StateRun,
	}, ServiceHelloWorldAPI)
	defer cancel()

	ctx.Log(log.LevelInfo, "starting to poll")

	var pollCount int
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-statesC:
			// Hello World API should have exited Run state, so we need to move out of run too.
			return nil

		case <-timer.C:
			if pollCount > s.maxPollCount {
				ctx.Log(log.LevelInfo, "reached maximum poll count, stopping")
				return nil
			}

			resp, err := s.client.Get(s.apiBase + "/api")
			if err != nil {
				// if we error, reset timer and try again...
				timer.Reset(s.retryDuration)
				continue
			}

			respBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				return err
				// we could return to new state: idle or stop or just continue
			}

			var respBody map[string]any
			err = json.Unmarshal(respBytes, &respBody)
			if err != nil {
				return err
				// we could return to new state: idle or stop or just continue to keep trying.
			}

			ctx.Log(log.LevelInfo, fmt.Sprintf("received response from the API: %v", respBody))
			// Increment polling counter
			pollCount++

			// Retry every 10s after the first time.
			timer.Reset(10 * time.Second)
		}
	}
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *APIPollingService) Stop(ctx rxd.ServiceContext) error {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	ctx.Log(log.LevelInfo, "stopping")
	return nil
}

func (s *APIPollingService) Init(ctx rxd.ServiceContext) error {
	ctx.Log(log.LevelInfo, "initializing")
	return nil
}

// Ensure we meet the interface or error.
var _ rxd.ServiceRunner = &APIPollingService{}
