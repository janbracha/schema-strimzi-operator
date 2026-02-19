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

	registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
)

var _ = Describe("SchemaRegistry Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		schemaregistry := &registryv1alpha1.SchemaRegistry{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind SchemaRegistry")
			err := k8sClient.Get(ctx, typeNamespacedName, schemaregistry)
			if err != nil && errors.IsNotFound(err) {
				resource := &registryv1alpha1.SchemaRegistry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: registryv1alpha1.SchemaRegistrySpec{
						URL:     "http://schema-registry.test.svc.cluster.local:8081",
						Timeout: 5,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &registryv1alpha1.SchemaRegistry{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SchemaRegistry")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SchemaRegistryReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// The reconciler performs a health check against a non-existent endpoint.
			// It should NOT return an error - instead it sets the status condition to
			// ConnectionFailed and requeues after 5 minutes.
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the status condition is set")
			updated := &registryv1alpha1.SchemaRegistry{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			Expect(updated.Status.ConnectionStatus).NotTo(BeEmpty())
		})
	})
})
