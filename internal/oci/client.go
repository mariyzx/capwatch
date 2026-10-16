package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

const (
	imageOS        = "Canonical Ubuntu"
	imageOSVersion = "22.04"
)

type Placement struct {
	AvailabilityDomain string
	SubnetID           string
	ImageID            string
}

type Client struct {
	compute  core.ComputeClient
	network  core.VirtualNetworkClient
	identity identity.IdentityClient
	tenancy  string
}

func NewClient() (*Client, error) {
	provider := common.DefaultConfigProvider()

	tenancy, err := provider.TenancyOCID()
	if err != nil {
		return nil, fmt.Errorf("reading tenancy from oci config: %w", err)
	}
	compute, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating compute client: %w", err)
	}
	network, err := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating network client: %w", err)
	}
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating identity client: %w", err)
	}

	return &Client{compute: compute, network: network, identity: identityClient, tenancy: tenancy}, nil
}

func (c *Client) Tenancy() string {
	return c.tenancy
}

func (c *Client) Discover(ctx context.Context, compartmentID, shape string) (Placement, error) {
	ads, err := c.identity.ListAvailabilityDomains(ctx, identity.ListAvailabilityDomainsRequest{
		CompartmentId: &compartmentID,
	})
	if err != nil {
		return Placement{}, fmt.Errorf("listing availability domains: %w", err)
	}
	ad, err := first(ads.Items, "availability domain")
	if err != nil {
		return Placement{}, err
	}

	subnets, err := c.network.ListSubnets(ctx, core.ListSubnetsRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(1),
	})
	if err != nil {
		return Placement{}, fmt.Errorf("listing subnets: %w", err)
	}
	subnet, err := first(subnets.Items, "subnet")
	if err != nil {
		return Placement{}, err
	}

	images, err := c.compute.ListImages(ctx, core.ListImagesRequest{
		CompartmentId:          &compartmentID,
		OperatingSystem:        common.String(imageOS),
		OperatingSystemVersion: common.String(imageOSVersion),
		Shape:                  &shape,
		SortBy:                 core.ListImagesSortByTimecreated,
		SortOrder:              core.ListImagesSortOrderDesc,
		Limit:                  common.Int(1),
	})
	if err != nil {
		return Placement{}, fmt.Errorf("listing images: %w", err)
	}
	image, err := first(images.Items, imageOS+" "+imageOSVersion+" image for "+shape)
	if err != nil {
		return Placement{}, err
	}

	return Placement{
		AvailabilityDomain: *ad.Name,
		SubnetID:           *subnet.Id,
		ImageID:            *image.Id,
	}, nil
}

func first[T any](items []T, what string) (T, error) {
	if len(items) == 0 {
		var zero T
		return zero, fmt.Errorf("no %s found", what)
	}
	return items[0], nil
}
