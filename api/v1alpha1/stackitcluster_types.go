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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// StackitNetworkSpec describes the network a StackitCluster runs its machines in.
type StackitNetworkSpec struct {
	// id is the ID of an existing STACKIT network to use for this cluster.
	// If empty, the controller creates a new isolated network named "<cluster-name>-network".
	// +optional
	ID *string `json:"id,omitempty"`
}

// StackitClusterSpec defines the desired state of StackitCluster
type StackitClusterSpec struct {
	// projectId is the STACKIT project ID that owns all resources created for this cluster.
	// +required
	// +kubebuilder:validation:MinLength=1
	ProjectID string `json:"projectId"`

	// region is the STACKIT region (e.g. "eu01") to create resources in.
	// +required
	// +kubebuilder:validation:MinLength=1
	Region string `json:"region"`

	// network configures the network used by this cluster's machines.
	// +optional
	Network StackitNetworkSpec `json:"network,omitempty"`

	// identityRef is a reference to a StackitClusterIdentity in the same namespace,
	// used to authenticate against the STACKIT API. If nil, the manager's ambient
	// STACKIT credentials (env vars / default credentials file) are used.
	// +optional
	IdentityRef *corev1.LocalObjectReference `json:"identityRef,omitempty"`

	// controlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// It is set by the StackitCluster controller once the control-plane public IP is reserved,
	// unless already set here to bring your own endpoint (e.g. an externally managed load balancer).
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`
}

// StackitClusterInitializationStatus provides observations of the StackitCluster provisioning process.
// NOTE: this struct is part of the Cluster API contract, and it is used to orchestrate provisioning.
type StackitClusterInitializationStatus struct {
	// provisioned is true when the infrastructure backing the cluster (network, security group,
	// control-plane public IP) has been fully created.
	// +optional
	Provisioned bool `json:"provisioned,omitempty"`
}

// StackitClusterStatus defines the observed state of StackitCluster.
type StackitClusterStatus struct {
	// initialization provides observations of the StackitCluster provisioning process.
	// +optional
	Initialization StackitClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// networkID is the ID of the network backing this cluster.
	// +optional
	NetworkID string `json:"networkID,omitempty"`

	// networkManaged is true if the network referenced by networkID was created by this
	// controller (and should be deleted when the cluster is deleted), false if it was
	// brought by the user via spec.network.id.
	// +optional
	NetworkManaged bool `json:"networkManaged,omitempty"`

	// securityGroupID is the ID of the security group applied to all machines in this cluster.
	// +optional
	SecurityGroupID string `json:"securityGroupID,omitempty"`

	// controlPlanePublicIPID is the ID of the public IP reserved for the control-plane endpoint.
	// +optional
	ControlPlanePublicIPID string `json:"controlPlanePublicIPID,omitempty"`

	// failureDomains is a list of failure domain objects synced from the STACKIT availability
	// zones available in spec.region.
	// +optional
	FailureDomains []clusterv1.FailureDomain `json:"failureDomains,omitempty"`

	// conditions represent the current state of the StackitCluster resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Ready": the resource is fully functional
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="cluster.x-k8s.io/v1beta2=v1alpha1"
// +kubebuilder:printcolumn:name="Provisioned",type="boolean",JSONPath=".status.initialization.provisioned"
// +kubebuilder:printcolumn:name="Network",type="string",JSONPath=".status.networkID"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint.host"

// StackitCluster is the Schema for the stackitclusters API
type StackitCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of StackitCluster
	// +required
	Spec StackitClusterSpec `json:"spec"`

	// status defines the observed state of StackitCluster
	// +optional
	Status StackitClusterStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// StackitClusterList contains a list of StackitCluster
type StackitClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []StackitCluster `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (c *StackitCluster) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *StackitCluster) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &StackitCluster{}, &StackitClusterList{})
		return nil
	})
}
