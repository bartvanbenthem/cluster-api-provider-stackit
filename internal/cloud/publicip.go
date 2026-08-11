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
)

// GetPublicIP returns the public IP with the given ID.
func (s *Scope) GetPublicIP(ctx context.Context, publicIPID string) (*iaas.PublicIp, error) {
	ip, err := s.API.GetPublicIP(ctx, s.ProjectID, s.Region, publicIPID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting public IP %q: %w", publicIPID, err)
	}
	return ip, nil
}

// CreatePublicIP reserves a new, unattached public IP for use as a cluster's
// control-plane endpoint.
func (s *Scope) CreatePublicIP(ctx context.Context) (*iaas.PublicIp, error) {
	ip, err := s.API.CreatePublicIP(ctx, s.ProjectID, s.Region).
		CreatePublicIPPayload(iaas.CreatePublicIPPayload{}).Execute()
	if err != nil {
		return nil, fmt.Errorf("creating public IP: %w", err)
	}
	return ip, nil
}

// DeletePublicIP deletes the public IP with the given ID. It is a no-op if the public IP
// no longer exists.
func (s *Scope) DeletePublicIP(ctx context.Context, publicIPID string) error {
	if err := s.API.DeletePublicIP(ctx, s.ProjectID, s.Region, publicIPID).Execute(); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting public IP %q: %w", publicIPID, err)
	}
	return nil
}

// AttachPublicIPToServer attaches the given public IP to the given server's primary
// network interface.
func (s *Scope) AttachPublicIPToServer(ctx context.Context, serverID, publicIPID string) error {
	if err := s.API.AddPublicIpToServer(ctx, s.ProjectID, s.Region, serverID, publicIPID).Execute(); err != nil {
		return fmt.Errorf("attaching public IP %q to server %q: %w", publicIPID, serverID, err)
	}
	return nil
}
