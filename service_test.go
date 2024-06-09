package rxd

import (
	"context"
	"testing"
)

var _ ServiceRunner = (*validService)(nil)

type validService struct{}

func (vs *validService) Init(ctx context.Context) error {
	return nil
}
func (vs *validService) Idle(ctx context.Context) error {
	return nil
}
func (vs *validService) Run(ctx context.Context) error {
	return nil
}
func (vs *validService) Stop(ctx context.Context) error {
	return nil
}

type invalidService struct{}

func (vs *invalidService) Init(ctx context.Context) error {
	return nil
}

// missing Idle
func (vs *invalidService) Run(ctx context.Context) error {
	return nil
}
func (vs *invalidService) Stop(ctx context.Context) error {
	return nil
}

func meetsInterface[T any](i T, n any) bool {
	_, ok := n.(T)
	return ok
}

func TestValidService(t *testing.T) {
	var vs any = &validService{}

	_, ok := vs.(Service)
	if !ok {
		t.Errorf("ValidService did not meet the Service interface to be a valid service")
	}
}

func TestInvalidService(t *testing.T) {
	var ivs any = &invalidService{}

	_, ok := ivs.(Service)

	if ok {
		t.Errorf("InvalidService should not not meet the Service interface but did")
	}
}
