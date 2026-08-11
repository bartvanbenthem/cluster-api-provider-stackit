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

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	iaas "github.com/stackitcloud/stackit-sdk-go/services/iaas/v2api"

	infrastructurev1alpha1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
	"github.com/bartvanbenthem/cluster-api-provider-stackit/internal/cloud"
)

// MachineFinalizer allows StackitMachineReconciler to clean up the backing STACKIT
// server before a StackitMachine is removed from the API server.
const MachineFinalizer = "stackitmachine.infrastructure.cluster.x-k8s.io"

// providerIDPrefix is the scheme used for StackitMachine provider IDs, in the form
// "stackit://<projectId>/<region>/<serverId>".
const providerIDPrefix = "stackit://"

// StackitMachineReconciler reconciles a StackitMachine object
type StackitMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile creates or removes the STACKIT server backing a StackitMachine, waiting for
// the owning Cluster's infrastructure and the Machine's bootstrap data to be ready
// before provisioning, per the Cluster API contract for InfrastructureMachine objects.
func (r *StackitMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	stackitMachine := &infrastructurev1alpha1.StackitMachine{}
	if err := r.Get(ctx, req.NamespacedName, stackitMachine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	machine, err := util.GetOwnerMachine(ctx, r.Client, stackitMachine.ObjectMeta)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting owner Machine: %w", err)
	}
	if machine == nil {
		log.Info("StackitMachine has no owner Machine yet, waiting")
		return ctrl.Result{}, nil
	}

	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label, waiting", "error", err.Error())
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, stackitMachine) {
		log.Info("StackitMachine or owner Cluster is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if !cluster.Spec.InfrastructureRef.IsDefined() {
		log.Info("Cluster has no infrastructureRef yet, waiting")
		return ctrl.Result{}, nil
	}
	stackitCluster := &infrastructurev1alpha1.StackitCluster{}
	stackitClusterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.InfrastructureRef.Name}
	if err := r.Get(ctx, stackitClusterKey, stackitCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting StackitCluster %q: %w", stackitClusterKey, err)
	}

	patchHelper, err := patch.NewHelper(stackitMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("creating patch helper: %w", err)
	}
	defer func() {
		if patchErr := patchHelper.Patch(ctx, stackitMachine, patch.WithOwnedConditions{Conditions: []string{ReadyConditionType}}); patchErr != nil {
			log.Error(patchErr, "failed to patch StackitMachine")
		}
	}()

	if !stackitMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, stackitCluster, stackitMachine)
	}

	if controllerutil.AddFinalizer(stackitMachine, MachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if !stackitCluster.Status.Initialization.Provisioned {
		log.Info("Waiting for StackitCluster infrastructure to be ready")
		conditions.Set(stackitMachine, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse,
			Reason: "WaitingForClusterInfrastructure",
		})
		return ctrl.Result{}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		log.Info("Waiting for bootstrap data secret to be ready")
		conditions.Set(stackitMachine, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse,
			Reason: "WaitingForBootstrapData",
		})
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, machine, stackitCluster, stackitMachine)
}

func (r *StackitMachineReconciler) reconcileNormal(
	ctx context.Context,
	machine *clusterv1.Machine,
	stackitCluster *infrastructurev1alpha1.StackitCluster,
	stackitMachine *infrastructurev1alpha1.StackitMachine,
) (ctrl.Result, error) {
	scope, err := cloud.NewScope(ctx, r.Client, stackitCluster)
	if err != nil {
		conditions.Set(stackitMachine, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse,
			Reason: "CredentialsInvalid", Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	var server *iaas.Server
	if stackitMachine.Spec.ProviderID == "" {
		server, err = r.createServer(ctx, scope, machine, stackitCluster, stackitMachine)
		if err != nil {
			conditions.Set(stackitMachine, metav1.Condition{
				Type: ReadyConditionType, Status: metav1.ConditionFalse,
				Reason: "ServerCreateFailed", Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		stackitMachine.Spec.ProviderID = fmt.Sprintf("%s%s/%s/%s", providerIDPrefix, scope.ProjectID, scope.Region, *server.Id)

		if util.IsControlPlaneMachine(machine) && stackitCluster.Status.ControlPlanePublicIPID != "" {
			ip, err := scope.GetPublicIP(ctx, stackitCluster.Status.ControlPlanePublicIPID)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !ip.NetworkInterface.IsSet() || ip.NetworkInterface.Get() == nil {
				if err := scope.AttachPublicIPToServer(ctx, *server.Id, stackitCluster.Status.ControlPlanePublicIPID); err != nil {
					return ctrl.Result{}, err
				}
				// Refresh so status.addresses below picks up the newly attached IP.
				server, err = r.getServer(ctx, scope, stackitMachine)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	} else {
		server, err = r.getServer(ctx, scope, stackitMachine)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.updateStatusFromServer(stackitMachine, server)
	return ctrl.Result{}, nil
}

func (r *StackitMachineReconciler) createServer(
	ctx context.Context,
	scope *cloud.Scope,
	machine *clusterv1.Machine,
	stackitCluster *infrastructurev1alpha1.StackitCluster,
	stackitMachine *infrastructurev1alpha1.StackitMachine,
) (*iaas.Server, error) {
	bootstrapData, err := r.getBootstrapData(ctx, machine)
	if err != nil {
		return nil, err
	}

	az := stackitMachine.Spec.AvailabilityZone
	if az == nil && machine.Spec.FailureDomain != "" {
		az = ptr.To(machine.Spec.FailureDomain)
	}

	server, err := scope.CreateServer(ctx, cloud.CreateServerParams{
		Name:             stackitMachine.Name,
		MachineType:      stackitMachine.Spec.MachineType,
		ImageID:          stackitMachine.Spec.ImageID,
		NetworkID:        stackitCluster.Status.NetworkID,
		SecurityGroupID:  stackitCluster.Status.SecurityGroupID,
		AvailabilityZone: az,
		RootDiskSizeGB:   stackitMachine.Spec.RootDiskSizeGB,
		KeypairName:      stackitMachine.Spec.SSHKeyName,
		UserData:         bootstrapData,
	})
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (r *StackitMachineReconciler) getServer(ctx context.Context, scope *cloud.Scope, stackitMachine *infrastructurev1alpha1.StackitMachine) (*iaas.Server, error) {
	serverID, err := serverIDFromProviderID(stackitMachine.Spec.ProviderID)
	if err != nil {
		return nil, err
	}
	return scope.GetServer(ctx, serverID)
}

func (r *StackitMachineReconciler) getBootstrapData(ctx context.Context, machine *clusterv1.Machine) ([]byte, error) {
	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: machine.Namespace, Name: *machine.Spec.Bootstrap.DataSecretName}
	if err := r.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("getting bootstrap data secret %q: %w", key, err)
	}
	value, ok := secret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("bootstrap data secret %q has no %q key", key, "value")
	}
	return value, nil
}

func (r *StackitMachineReconciler) updateStatusFromServer(stackitMachine *infrastructurev1alpha1.StackitMachine, server *iaas.Server) {
	stackitMachine.Status.InstanceState = server.Status

	addresses := []clusterv1.MachineAddress{}
	for _, nic := range server.Nics {
		if nic.Ipv4 != nil {
			addresses = append(addresses, clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
				Address: *nic.Ipv4,
			})
		}
		if nic.PublicIp != nil {
			addresses = append(addresses, clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: *nic.PublicIp,
			})
		}
	}
	stackitMachine.Status.Addresses = addresses

	if server.AvailabilityZone != nil {
		stackitMachine.Status.FailureDomain = *server.AvailabilityZone
	}

	switch {
	case server.Status != nil && cloud.ServerActiveStates[*server.Status]:
		stackitMachine.Status.Initialization.Provisioned = true
		conditions.Set(stackitMachine, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionTrue, Reason: "ServerActive",
		})
	case server.Status != nil && cloud.ServerErrorStates[*server.Status]:
		stackitMachine.Status.FailureReason = ptr.To("ServerError")
		stackitMachine.Status.FailureMessage = server.ErrorMessage
		conditions.Set(stackitMachine, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse, Reason: "ServerError",
		})
	default:
		conditions.Set(stackitMachine, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse, Reason: "ServerProvisioning",
		})
	}
}

func (r *StackitMachineReconciler) reconcileDelete(ctx context.Context, stackitCluster *infrastructurev1alpha1.StackitCluster, stackitMachine *infrastructurev1alpha1.StackitMachine) (ctrl.Result, error) {
	if stackitMachine.Spec.ProviderID != "" {
		scope, err := cloud.NewScope(ctx, r.Client, stackitCluster)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("building cloud scope for deletion: %w", err)
		}

		serverID, err := serverIDFromProviderID(stackitMachine.Spec.ProviderID)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := scope.DeleteServer(ctx, serverID); err != nil {
			return ctrl.Result{}, err
		}
	}

	controllerutil.RemoveFinalizer(stackitMachine, MachineFinalizer)
	return ctrl.Result{}, nil
}

// serverIDFromProviderID extracts the STACKIT server ID from a provider ID of the form
// "stackit://<projectId>/<region>/<serverId>".
func serverIDFromProviderID(providerID string) (string, error) {
	trimmed, ok := strings.CutPrefix(providerID, providerIDPrefix)
	if !ok {
		return "", fmt.Errorf("invalid StackitMachine providerID %q: missing %q prefix", providerID, providerIDPrefix)
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 || parts[2] == "" {
		return "", fmt.Errorf("invalid StackitMachine providerID %q", providerID)
	}
	return parts[2], nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StackitMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gvk := infrastructurev1alpha1.GroupVersion.WithKind("StackitMachine")

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.StackitMachine{}).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(gvk)),
		).
		Named("stackitmachine").
		Complete(r)
}
