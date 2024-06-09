package rxd

import (
	"fmt"
	"testing"
)

func TestNewServiceResponse(t *testing.T) {
	err := fmt.Errorf("testing")
	// defaultOpts contains default runPolicy of RunUntilStoppedPolicy
	response := NewResponse(err, StateInit)

	wantState := StateInit
	wantError := err

	if response.NextState != wantState {
		t.Errorf("NewServiceResponse did not store correct value for next state, want %s - got %s", wantState, response.NextState)
	}

	if response.Error != wantError {
		t.Errorf("NewServiceResponse did not store correct value for the error, want %s - got %s", wantError, response.Error)
	}
}
