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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrastructurev1alpha1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
)

var _ = Describe("StackitClusterIdentity Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			secretName        = "test-resource-credentials"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		stackitclusteridentity := &infrastructurev1alpha1.StackitClusterIdentity{}

		BeforeEach(func() {
			By("creating the credentials Secret referenced by the StackitClusterIdentity")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: resourceNamespace,
				},
				Data: map[string][]byte{
					infrastructurev1alpha1.StackitSecretKeyServiceAccountKey: []byte(`{"some":"key"}`),
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: resourceNamespace}, &corev1.Secret{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}

			By("creating the custom resource for the Kind StackitClusterIdentity")
			err = k8sClient.Get(ctx, typeNamespacedName, stackitclusteridentity)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1alpha1.StackitClusterIdentity{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: infrastructurev1alpha1.StackitClusterIdentitySpec{
						SecretRef: corev1.LocalObjectReference{Name: secretName},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &infrastructurev1alpha1.StackitClusterIdentity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance StackitClusterIdentity")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: resourceNamespace}, secret)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})
		It("should mark the resource Ready when the referenced Secret carries valid credential keys", func() {
			By("Reconciling the created resource")
			controllerReconciler := &StackitClusterIdentityReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &infrastructurev1alpha1.StackitClusterIdentity{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			Expect(updated.Status.Conditions).To(ContainElement(
				HaveField("Type", Equal(ReadyConditionType)),
			))
			Expect(updated.GetConditions()[0].Status).To(Equal(metav1.ConditionTrue))
		})
	})
})
