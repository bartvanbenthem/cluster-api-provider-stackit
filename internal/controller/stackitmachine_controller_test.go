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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrastructurev1alpha1 "github.com/bartvanbenthem/cluster-api-provider-stackit/api/v1alpha1"
)

var _ = Describe("StackitMachine Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		stackitmachine := &infrastructurev1alpha1.StackitMachine{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind StackitMachine")
			err := k8sClient.Get(ctx, typeNamespacedName, stackitmachine)
			if err != nil && errors.IsNotFound(err) {
				resource := &infrastructurev1alpha1.StackitMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: infrastructurev1alpha1.StackitMachineSpec{
						MachineType: "g1.2",
						ImageID:     "11111111-1111-1111-1111-111111111111",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &infrastructurev1alpha1.StackitMachine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance StackitMachine")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should wait without error when the StackitMachine has no owner Machine yet", func() {
			// This StackitMachine has no owner Machine, so Reconcile must return early
			// without touching the STACKIT API (no credentials are configured in this
			// test environment). Full provisioning behavior needs a real or mocked
			// STACKIT API and is exercised via internal/cloud's unit tests instead.
			By("Reconciling the created resource")
			controllerReconciler := &StackitMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
