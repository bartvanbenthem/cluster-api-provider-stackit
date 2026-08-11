/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"
	"fmt"

	iaas "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"
	"github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api/wait"
)

// GetNetwork returns the network with the given ID.
func (s *Scope) GetNetwork(ctx context.Context, networkID string) (*iaas.Network, error) {
	network, err := s.API.GetNetwork(ctx, s.ProjectID, s.Region, networkID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting network %q: %w", networkID, err)
	}
	return network, nil
}

// CreateNetwork creates a new isolated /24 network with the given name and waits for it
// to become active.
func (s *Scope) CreateNetwork(ctx context.Context, name string) (*iaas.Network, error) {
	payload := iaas.CreateNetworkPayload{
		Name: name,
		Ipv4: &iaas.CreateNetworkIPv4{
			CreateNetworkIPv4WithPrefixLength: &iaas.CreateNetworkIPv4WithPrefixLength{
				PrefixLength: 24,
			},
		},
	}

	network, err := s.API.CreateNetwork(ctx, s.ProjectID, s.Region).CreateNetworkPayload(payload).Execute()
	if err != nil {
		return nil, fmt.Errorf("creating network %q: %w", name, err)
	}

	network, err = wait.CreateNetworkWaitHandler(ctx, s.API, s.ProjectID, s.Region, network.Id).WaitWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for network %q to become active: %w", name, err)
	}
	return network, nil
}

// DeleteNetwork deletes the network with the given ID and waits for the deletion to
// complete. It is a no-op if the network no longer exists.
func (s *Scope) DeleteNetwork(ctx context.Context, networkID string) error {
	if err := s.API.DeleteNetwork(ctx, s.ProjectID, s.Region, networkID).Execute(); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting network %q: %w", networkID, err)
	}

	if _, err := wait.DeleteNetworkWaitHandler(ctx, s.API, s.ProjectID, s.Region, networkID).WaitWithContext(ctx); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("waiting for network %q to be deleted: %w", networkID, err)
	}
	return nil
}
