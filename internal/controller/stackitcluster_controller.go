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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"

	infrastructurev1alpha1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
	"github.com/bartvanbenthem/cluster-api-provider-stackit/internal/cloud"
)

// ClusterFinalizer allows StackitClusterReconciler to clean up STACKIT resources before
// a StackitCluster is removed from the API server.
const ClusterFinalizer = "stackitcluster.infrastructure.cluster.x-k8s.io"

// StackitClusterReconciler reconciles a StackitCluster object
type StackitClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusteridentities,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile creates or removes the STACKIT network, security group and control-plane
// public IP backing a StackitCluster, keeping status and the resolved control-plane
// endpoint in sync with the Cluster API contract for InfrastructureCluster objects.
func (r *StackitClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	stackitCluster := &infrastructurev1alpha1.StackitCluster{}
	if err := r.Get(ctx, req.NamespacedName, stackitCluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, stackitCluster.ObjectMeta)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting owner Cluster: %w", err)
	}
	if cluster == nil {
		log.Info("StackitCluster has no owner Cluster yet, waiting")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, stackitCluster) {
		log.Info("StackitCluster or owner Cluster is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(stackitCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("creating patch helper: %w", err)
	}
	defer func() {
		if patchErr := patchHelper.Patch(ctx, stackitCluster, patch.WithOwnedConditions{Conditions: []string{ReadyConditionType}}); patchErr != nil {
			log.Error(patchErr, "failed to patch StackitCluster")
		}
	}()

	if !stackitCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, stackitCluster)
	}

	if controllerutil.AddFinalizer(stackitCluster, ClusterFinalizer) {
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, stackitCluster)
}

func (r *StackitClusterReconciler) reconcileNormal(ctx context.Context, stackitCluster *infrastructurev1alpha1.StackitCluster) (ctrl.Result, error) {
	scope, err := cloud.NewScope(ctx, r.Client, stackitCluster)
	if err != nil {
		conditions.Set(stackitCluster, metav1.Condition{
			Type:    ReadyConditionType,
			Status:  metav1.ConditionFalse,
			Reason:  "CredentialsInvalid",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	if err := r.reconcileNetwork(ctx, scope, stackitCluster); err != nil {
		conditions.Set(stackitCluster, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse,
			Reason: "NetworkReconcileFailed", Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	if err := r.reconcileSecurityGroup(ctx, scope, stackitCluster); err != nil {
		conditions.Set(stackitCluster, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse,
			Reason: "SecurityGroupReconcileFailed", Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	if err := r.reconcileControlPlaneEndpoint(ctx, scope, stackitCluster); err != nil {
		conditions.Set(stackitCluster, metav1.Condition{
			Type: ReadyConditionType, Status: metav1.ConditionFalse,
			Reason: "ControlPlaneEndpointReconcileFailed", Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	failureDomains, err := scope.ListFailureDomains(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing failure domains: %w", err)
	}
	stackitCluster.Status.FailureDomains = failureDomains

	stackitCluster.Status.Initialization.Provisioned = true
	conditions.Set(stackitCluster, metav1.Condition{
		Type:   ReadyConditionType,
		Status: metav1.ConditionTrue,
		Reason: "Provisioned",
	})
	return ctrl.Result{}, nil
}

func (r *StackitClusterReconciler) reconcileNetwork(ctx context.Context, scope *cloud.Scope, stackitCluster *infrastructurev1alpha1.StackitCluster) error {
	if stackitCluster.Status.NetworkID != "" {
		return nil
	}

	if stackitCluster.Spec.Network.ID != nil && *stackitCluster.Spec.Network.ID != "" {
		if _, err := scope.GetNetwork(ctx, *stackitCluster.Spec.Network.ID); err != nil {
			return fmt.Errorf("adopting network %q: %w", *stackitCluster.Spec.Network.ID, err)
		}
		stackitCluster.Status.NetworkID = *stackitCluster.Spec.Network.ID
		stackitCluster.Status.NetworkManaged = false
		return nil
	}

	network, err := scope.CreateNetwork(ctx, fmt.Sprintf("%s-network", stackitCluster.Name))
	if err != nil {
		return err
	}
	stackitCluster.Status.NetworkID = network.Id
	stackitCluster.Status.NetworkManaged = true
	return nil
}

func (r *StackitClusterReconciler) reconcileSecurityGroup(ctx context.Context, scope *cloud.Scope, stackitCluster *infrastructurev1alpha1.StackitCluster) error {
	if stackitCluster.Status.SecurityGroupID != "" {
		return nil
	}

	sg, err := scope.CreateClusterSecurityGroup(ctx, fmt.Sprintf("%s-cluster", stackitCluster.Name))
	if err != nil {
		return err
	}
	stackitCluster.Status.SecurityGroupID = *sg.Id
	return nil
}

func (r *StackitClusterReconciler) reconcileControlPlaneEndpoint(ctx context.Context, scope *cloud.Scope, stackitCluster *infrastructurev1alpha1.StackitCluster) error {
	if stackitCluster.Spec.ControlPlaneEndpoint.IsValid() {
		// Bring-your-own endpoint (e.g. an externally managed load balancer): nothing to do.
		return nil
	}

	if stackitCluster.Status.ControlPlanePublicIPID == "" {
		ip, err := scope.CreatePublicIP(ctx)
		if err != nil {
			return err
		}
		stackitCluster.Status.ControlPlanePublicIPID = *ip.Id
		stackitCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
			Host: *ip.Ip,
			Port: 6443,
		}
	}
	return nil
}

func (r *StackitClusterReconciler) reconcileDelete(ctx context.Context, stackitCluster *infrastructurev1alpha1.StackitCluster) (ctrl.Result, error) {
	scope, err := cloud.NewScope(ctx, r.Client, stackitCluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building cloud scope for deletion: %w", err)
	}

	if stackitCluster.Status.ControlPlanePublicIPID != "" {
		if err := scope.DeletePublicIP(ctx, stackitCluster.Status.ControlPlanePublicIPID); err != nil {
			return ctrl.Result{}, err
		}
	}

	if stackitCluster.Status.SecurityGroupID != "" {
		if err := scope.DeleteSecurityGroup(ctx, stackitCluster.Status.SecurityGroupID); err != nil {
			return ctrl.Result{}, err
		}
	}

	if stackitCluster.Status.NetworkManaged && stackitCluster.Status.NetworkID != "" {
		if err := scope.DeleteNetwork(ctx, stackitCluster.Status.NetworkID); err != nil {
			return ctrl.Result{}, err
		}
	}

	controllerutil.RemoveFinalizer(stackitCluster, ClusterFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StackitClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gvk := infrastructurev1alpha1.GroupVersion.WithKind("StackitCluster")

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.StackitCluster{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(context.Background(), gvk, mgr.GetClient(), &infrastructurev1alpha1.StackitCluster{})),
			builder.WithPredicates(predicates.ClusterUnpaused(mgr.GetScheme(), logf.Log)),
		).
		Named("stackitcluster").
		Complete(r)
}
