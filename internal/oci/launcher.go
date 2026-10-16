package oci

import (
	"context"
	"errors"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/mariyzx/capwatch/internal/hunt"
)

type Spec struct {
	Placement
	CompartmentID string
	Shape         string
	OCPUs         float32
	MemoryGB      float32
	BootVolumeGB  int64
	DisplayName   string
	SSHPublicKey  string
}

type instanceLauncher interface {
	LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error)
}

type Launcher struct {
	compute    instanceLauncher
	spec       Spec
	retryToken string
}

func (c *Client) Launcher(spec Spec) *Launcher {
	return &Launcher{compute: c.compute, spec: spec}
}

func (l *Launcher) Launch(ctx context.Context) (hunt.Instance, error) {
	if l.retryToken == "" {
		l.retryToken = common.RetryToken()
	}

	response, err := l.compute.LaunchInstance(ctx, l.request())
	if err != nil {
		err = classify(err)
		// A timed out launch may still succeed on OCI's side. Reusing the token
		// makes OCI return that instance instead of creating a second one.
		if !errors.Is(err, hunt.ErrTransient) {
			l.retryToken = ""
		}
		return hunt.Instance{}, fmt.Errorf("launching instance: %w", err)
	}

	l.retryToken = ""
	return hunt.Instance{ID: *response.Id}, nil
}

func (l *Launcher) request() core.LaunchInstanceRequest {
	s := l.spec
	noRetry := common.NoRetryPolicy()

	return core.LaunchInstanceRequest{
		LaunchInstanceDetails: core.LaunchInstanceDetails{
			AvailabilityDomain: &s.AvailabilityDomain,
			CompartmentId:      &s.CompartmentID,
			Shape:              &s.Shape,
			DisplayName:        &s.DisplayName,
			ShapeConfig: &core.LaunchInstanceShapeConfigDetails{
				Ocpus:       &s.OCPUs,
				MemoryInGBs: &s.MemoryGB,
			},
			SourceDetails: core.InstanceSourceViaImageDetails{
				ImageId:             &s.ImageID,
				BootVolumeSizeInGBs: &s.BootVolumeGB,
			},
			CreateVnicDetails: &core.CreateVnicDetails{
				SubnetId:       &s.SubnetID,
				AssignPublicIp: common.Bool(true),
			},
			Metadata: map[string]string{"ssh_authorized_keys": s.SSHPublicKey},
		},
		OpcRetryToken:   common.String(l.retryToken),
		RequestMetadata: common.RequestMetadata{RetryPolicy: &noRetry},
	}
}
