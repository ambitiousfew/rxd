package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ambitiousfew/rxd"
)

// APIPollingService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type APIPollingService struct {
	// fields this specific server uses
	client        *http.Client
	apiBase       string
	retryDuration time.Duration
	maxPollCount  int
	log           rxd.Logger
}

// NewAPIPollingService just a factory helper function to help create and return a new instance of the service.
func NewAPIPollingService(log rxd.Logger) *APIPollingService {
	return &APIPollingService{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		// We will check every 10s to see if we can establish a connection to the API when Idle retrying.
		retryDuration: 10 * time.Second,
		apiBase:       "http://localhost:8000",
		maxPollCount:  5,
		log:           log,
	}
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *APIPollingService) Idle(ctx context.Context) error {

	// APIPolling service is registering its interest in ALL services passed ENTERING a "RunState"
	// So if HelloWorldAPI is the only passed service here and it ENTERS a "RunState" then
	//  APIPolling service will be notified of that state change.
	// This is how we are able to listen to state changes of other services running within RxD.
	// enteredStateC, cancel := rxd.AllServicesEnterState(sc, rxd.RunState, HelloWorldAPI)
	// defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		// case <-enteredStateC:
		default:
			// if we receive a state change over this channel, it will only happen
			// because ALL services we are interested in have entered their run state.
			// HelloWorldAPI should be running, APIPolling can now move from
			// Idle to Run state.

			// We must exit Idle and specify the next state we want to enter.
			return nil
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *APIPollingService) Run(ctx context.Context) error {
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	// Here we are registering our interest in ANY of the services passed EXITING a "RunState"
	// So if any service given here for some reasons LEAVES their RunState, we will be notified.

	// exitStateC, cancel := rxd.AnyServicesExitState(sc, rxd.RunState, HelloWorldAPI)
	// defer cancel()

	s.log.Info("starting to poll")

	var pollCount int
	for {
		select {
		case <-ctx.Done():
			return nil
		// case <-exitStateC:
		// Polling service can wait to be Notified of a specific state change, or even a state to be put into.
		// return nil

		case <-timer.C:
			if pollCount > s.maxPollCount {
				s.log.Info("reached maximum poll count, stopping")
				return nil
			}

			resp, err := s.client.Get(s.apiBase + "/api")
			if err != nil {
				return err
				// if we error, reset timer and try again...
				// timer.Reset(s.retryDuration)
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

			s.log.Info(fmt.Sprintf("received response from the API: %v", respBody))
			// Increment polling counter
			pollCount++

			// Retry every 10s after the first time.
			timer.Reset(10 * time.Second)
		}
	}
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *APIPollingService) Stop(ctx context.Context) error {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	s.log.Info("stopping")
	return nil
}

func (s *APIPollingService) Init(ctx context.Context) error {
	s.log.Info("initializing")
	return nil
}

// Ensure we meet the interface or error.
var _ rxd.ServiceRunner = &APIPollingService{}
