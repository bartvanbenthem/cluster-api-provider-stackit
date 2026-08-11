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

	"github.com/stackitcloud/stackit-sdk-go/core/utils"
	iaas "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"
)

const ingressDirection = "ingress"

// GetSecurityGroup returns the security group with the given ID.
func (s *Scope) GetSecurityGroup(ctx context.Context, securityGroupID string) (*iaas.SecurityGroup, error) {
	sg, err := s.API.GetSecurityGroup(ctx, s.ProjectID, s.Region, securityGroupID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting security group %q: %w", securityGroupID, err)
	}
	return sg, nil
}

// CreateClusterSecurityGroup creates a security group named after the cluster with the
// baseline rules every cluster machine needs: SSH and Kubernetes API access from
// anywhere, unrestricted traffic between members of the group (kubelet, etcd, CNI
// overlay, NodePort services, ...), and unrestricted egress.
func (s *Scope) CreateClusterSecurityGroup(ctx context.Context, name string) (*iaas.SecurityGroup, error) {
	payload := iaas.CreateSecurityGroupPayload{
		Name:        name,
		Description: utils.Ptr("Managed by cluster-api-provider-stackit. Do not edit rules directly."),
	}

	sg, err := s.API.CreateSecurityGroup(ctx, s.ProjectID, s.Region).CreateSecurityGroupPayload(payload).Execute()
	if err != nil {
		return nil, fmt.Errorf("creating security group %q: %w", name, err)
	}

	if err := s.createSecurityGroupRules(ctx, *sg.Id); err != nil {
		return nil, err
	}
	return sg, nil
}

func (s *Scope) createSecurityGroupRules(ctx context.Context, securityGroupID string) error {
	tcp := iaas.StringAsCreateProtocol(utils.Ptr("tcp"))

	rules := []iaas.CreateSecurityGroupRulePayload{
		{
			Direction: ingressDirection,
			Ethertype: utils.Ptr("IPv4"),
			Protocol:  &tcp,
			IpRange:   utils.Ptr("0.0.0.0/0"),
			PortRange: &iaas.PortRange{Min: 22, Max: 22},
		},
		{
			Direction: ingressDirection,
			Ethertype: utils.Ptr("IPv4"),
			Protocol:  &tcp,
			IpRange:   utils.Ptr("0.0.0.0/0"),
			PortRange: &iaas.PortRange{Min: 6443, Max: 6443},
		},
		{
			// Allow all traffic between members of this security group (kubelet,
			// etcd, CNI overlay, NodePort services, ...).
			Direction:             ingressDirection,
			Ethertype:             utils.Ptr("IPv4"),
			RemoteSecurityGroupId: utils.Ptr(securityGroupID),
		},
		{
			// Unrestricted egress.
			Direction: "egress",
			Ethertype: utils.Ptr("IPv4"),
			IpRange:   utils.Ptr("0.0.0.0/0"),
		},
	}

	for _, rule := range rules {
		if _, err := s.API.CreateSecurityGroupRule(ctx, s.ProjectID, s.Region, securityGroupID).
			CreateSecurityGroupRulePayload(rule).Execute(); err != nil {
			return fmt.Errorf("creating %s rule in security group %q: %w", rule.Direction, securityGroupID, err)
		}
	}
	return nil
}

// DeleteSecurityGroup deletes the security group with the given ID. It is a no-op if the
// security group no longer exists.
func (s *Scope) DeleteSecurityGroup(ctx context.Context, securityGroupID string) error {
	if err := s.API.DeleteSecurityGroup(ctx, s.ProjectID, s.Region, securityGroupID).Execute(); err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("deleting security group %q: %w", securityGroupID, err)
	}
	return nil
}
