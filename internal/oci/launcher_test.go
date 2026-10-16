package oci

import (
	"context"
	"errors"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/mariyzx/capwatch/internal/hunt"
)

type fakeCompute struct {
	errs     []error
	requests []core.LaunchInstanceRequest
}

func (f *fakeCompute) LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
	f.requests = append(f.requests, request)
	if len(f.errs) == 0 {
		return core.LaunchInstanceResponse{Instance: core.Instance{Id: common.String("ocid1.instance.test")}}, nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return core.LaunchInstanceResponse{}, err
}

func (f *fakeCompute) token(i int) string {
	return *f.requests[i].OpcRetryToken
}

func TestLaunchReturnsInstance(t *testing.T) {
	launcher := &Launcher{compute: &fakeCompute{}, spec: Spec{Shape: "VM.Standard.A1.Flex"}}

	instance, err := launcher.Launch(context.Background())

	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	if instance.ID != "ocid1.instance.test" {
		t.Errorf("instance.ID = %q", instance.ID)
	}
}

func TestLaunchClassifiesErrors(t *testing.T) {
	compute := &fakeCompute{errs: []error{fakeServiceError{500, "InternalError", "Out of host capacity."}}}
	launcher := &Launcher{compute: compute}

	_, err := launcher.Launch(context.Background())

	if !errors.Is(err, hunt.ErrOutOfCapacity) {
		t.Errorf("Launch() error = %v, want %v", err, hunt.ErrOutOfCapacity)
	}
}

func TestLaunchRetryToken(t *testing.T) {
	compute := &fakeCompute{errs: []error{
		fakeNetworkError{},
		fakeServiceError{500, "InternalError", "Out of host capacity."},
	}}
	launcher := &Launcher{compute: compute}

	for range 3 {
		launcher.Launch(context.Background())
	}

	if compute.token(0) != compute.token(1) {
		t.Error("token changed after a transient error, want it reused")
	}
	if compute.token(1) == compute.token(2) {
		t.Error("token reused after out of capacity, want a new one")
	}
}
