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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// StackitMachineSpec defines the desired state of StackitMachine
type StackitMachineSpec struct {
	// providerID is the identification ID of the server as understood by the STACKIT provider.
	// It is set by the controller once the server is created, in the form
	// "stackit://<projectId>/<region>/<serverId>".
	// +optional
	ProviderID string `json:"providerID,omitempty"`

	// machineType is the STACKIT machine type (flavor) to use, e.g. "g1.2".
	// +required
	// +kubebuilder:validation:MinLength=1
	MachineType string `json:"machineType"`

	// imageId is the ID of the STACKIT image (e.g. an Ubuntu image) used as the boot volume source.
	// +required
	// +kubebuilder:validation:MinLength=1
	ImageID string `json:"imageId"`

	// rootDiskSizeGB is the size in GB of the boot volume. Defaults to 32 if unset.
	// +optional
	// +kubebuilder:validation:Minimum=1
	RootDiskSizeGB *int64 `json:"rootDiskSizeGB,omitempty"`

	// sshKeyName is the name of an existing STACKIT SSH key pair to inject into the server.
	// +optional
	SSHKeyName *string `json:"sshKeyName,omitempty"`

	// availabilityZone pins the server to a specific STACKIT availability zone. If unset, the
	// value is taken from the owning Machine's spec.failureDomain.
	// +optional
	AvailabilityZone *string `json:"availabilityZone,omitempty"`
}

// StackitMachineInitializationStatus provides observations of the StackitMachine provisioning process.
// NOTE: this struct is part of the Cluster API contract, and it is used to orchestrate provisioning.
type StackitMachineInitializationStatus struct {
	// provisioned is true when the backing STACKIT server has been created and is running.
	// +optional
	Provisioned bool `json:"provisioned,omitempty"`
}

// StackitMachineStatus defines the observed state of StackitMachine.
type StackitMachineStatus struct {
	// initialization provides observations of the StackitMachine provisioning process.
	// +optional
	Initialization StackitMachineInitializationStatus `json:"initialization,omitempty,omitzero"`

	// addresses contains the associated addresses for the backing STACKIT server.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// failureDomain is the failure domain (availability zone) the server was actually created in.
	// +optional
	FailureDomain string `json:"failureDomain,omitempty"`

	// instanceState is the last observed STACKIT server status (e.g. "CREATING", "ACTIVE", "ERROR").
	// +optional
	InstanceState *string `json:"instanceState,omitempty"`

	// failureReason will be set in the event that there is a terminal problem reconciling the machine.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// failureMessage will be set in the event that there is a terminal problem reconciling the machine.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// conditions represent the current state of the StackitMachine resource.
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
// +kubebuilder:printcolumn:name="InstanceState",type="string",JSONPath=".status.instanceState"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",priority=10

// StackitMachine is the Schema for the stackitmachines API
type StackitMachine struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of StackitMachine
	// +required
	Spec StackitMachineSpec `json:"spec"`

	// status defines the observed state of StackitMachine
	// +optional
	Status StackitMachineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// StackitMachineList contains a list of StackitMachine
type StackitMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []StackitMachine `json:"items"`
}

// GetConditions returns the set of conditions for this object.
func (m *StackitMachine) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (m *StackitMachine) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &StackitMachine{}, &StackitMachineList{})
		return nil
	})
}
