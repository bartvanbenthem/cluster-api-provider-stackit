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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrastructurev1alpha1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
)

// ReadyConditionType is the condition type used to summarize whether a resource has
// finished reconciling successfully.
const ReadyConditionType = "Ready"

// StackitClusterIdentityReconciler reconciles a StackitClusterIdentity object
type StackitClusterIdentityReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusteridentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=stackitclusteridentities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates that the Secret referenced by a StackitClusterIdentity exists and
// carries one of the supported credential keys, and reflects the result in the Ready
// condition. It performs no external STACKIT API calls.
func (r *StackitClusterIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	identity := &infrastructurev1alpha1.StackitClusterIdentity{}
	if err := r.Get(ctx, req.NamespacedName, identity); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(identity, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("creating patch helper: %w", err)
	}
	defer func() {
		if patchErr := patchHelper.Patch(ctx, identity, patch.WithOwnedConditions{Conditions: []string{ReadyConditionType}}); patchErr != nil {
			log.Error(patchErr, "failed to patch StackitClusterIdentity")
		}
	}()

	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{Namespace: identity.Namespace, Name: identity.Spec.SecretRef.Name}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(identity, metav1.Condition{
				Type:    ReadyConditionType,
				Status:  metav1.ConditionFalse,
				Reason:  "SecretNotFound",
				Message: fmt.Sprintf("secret %q not found", secretKey.Name),
			})
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting secret %q: %w", secretKey, err)
	}

	hasKey := len(secret.Data[infrastructurev1alpha1.StackitSecretKeyServiceAccountKey]) > 0
	hasToken := len(secret.Data[infrastructurev1alpha1.StackitSecretKeyToken]) > 0
	if !hasKey && !hasToken {
		conditions.Set(identity, metav1.Condition{
			Type:   ReadyConditionType,
			Status: metav1.ConditionFalse,
			Reason: "MissingCredentialKey",
			Message: fmt.Sprintf("secret %q must set data key %q or %q", secretKey.Name,
				infrastructurev1alpha1.StackitSecretKeyServiceAccountKey, infrastructurev1alpha1.StackitSecretKeyToken),
		})
		return ctrl.Result{}, nil
	}

	conditions.Set(identity, metav1.Condition{
		Type:   ReadyConditionType,
		Status: metav1.ConditionTrue,
		Reason: "CredentialsFound",
	})
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StackitClusterIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.StackitClusterIdentity{}).
		Named("stackitclusteridentity").
		Complete(r)
}
