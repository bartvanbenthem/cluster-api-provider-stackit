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
	"encoding/base64"
	"fmt"

	"github.com/stackitcloud/stackit-sdk-go/core/utils"
	iaas "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"
	"github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api/wait"
)

// ServerActiveStates are the STACKIT server statuses that count as "provisioned" for the
// purposes of the InfrastructureMachine contract, i.e. the server is up and reachable.
var ServerActiveStates = map[string]bool{
	"ACTIVE": true,
}

// ServerErrorStates are the STACKIT server statuses that indicate a terminal failure.
var ServerErrorStates = map[string]bool{
	"ERROR": true,
}

// CreateServerParams holds the inputs needed to create a STACKIT server backing a
// StackitMachine.
type CreateServerParams struct {
	Name             string
	MachineType      string
	ImageID          string
	NetworkID        string
	SecurityGroupID  string
	AvailabilityZone *string
	RootDiskSizeGB   *int64
	KeypairName      *string
	// UserData is the raw (non-base64-encoded) cloud-init user data.
	UserData []byte
}

// GetServer returns the server with the given ID.
func (s *Scope) GetServer(ctx context.Context, serverID string) (*iaas.Server, error) {
	server, err := s.API.GetServer(ctx, s.ProjectID, s.Region, serverID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting server %q: %w", serverID, err)
	}
	return server, nil
}

// CreateServer creates a server and waits for it to leave the CREATING state.
func (s *Scope) CreateServer(ctx context.Context, params CreateServerParams) (*iaas.Server, error) {
	rootDiskSize := int64(32)
	if params.RootDiskSizeGB != nil {
		rootDiskSize = *params.RootDiskSizeGB
	}

	payload := iaas.CreateServerPayload{
		Name:             params.Name,
		MachineType:      params.MachineType,
		AvailabilityZone: params.AvailabilityZone,
		KeypairName:      params.KeypairName,
		SecurityGroups:   []string{params.SecurityGroupID},
		BootVolume: &iaas.BootVolume{
			Size: utils.Ptr(rootDiskSize),
			Source: &iaas.BootVolumeSource{
				Id:   params.ImageID,
				Type: "image",
			},
		},
		Networking: iaas.CreateServerPayloadAllOfNetworking{
			CreateServerNetworking: &iaas.CreateServerNetworking{
				NetworkId: utils.Ptr(params.NetworkID),
			},
		},
	}
	if len(params.UserData) > 0 {
		payload.UserData = utils.Ptr(base64.StdEncoding.EncodeToString(params.UserData))
	}

	server, err := s.API.CreateServer(ctx, s.ProjectID, s.Region).CreateServerPayload(payload).Execute()
	if err != nil {
		return nil, fmt.Errorf("creating server %q: %w", params.Name, err)
	}

	server, err = wait.CreateServerWaitHandler(ctx, s.API, s.ProjectID, s.Region, *server.Id).WaitWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("waiting for server %q to be created: %w", params.Name, err)
	}
	return server, nil
}

// DeleteServer deletes the server with the given ID and waits for the deletion to
// complete. It is a no-op if the server no longer exists.
func (s *Scope) DeleteServer(ctx context.Context, serverID string) error {
	if err := s.API.DeleteServer(ctx, s.ProjectID, s.Region, serverID).Execute(); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting server %q: %w", serverID, err)
	}

	if _, err := wait.DeleteServerWaitHandler(ctx, s.API, s.ProjectID, s.Region, serverID).WaitWithContext(ctx); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("waiting for server %q to be deleted: %w", serverID, err)
	}
	return nil
}
