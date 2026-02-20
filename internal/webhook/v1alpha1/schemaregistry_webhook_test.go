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
. "github.com/onsi/ginkgo/v2"
. "github.com/onsi/gomega"

metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
)

func validSchemaRegistry() *registryv1alpha1.SchemaRegistry {
return &registryv1alpha1.SchemaRegistry{
ObjectMeta: metav1.ObjectMeta{Name: "test-registry", Namespace: "default"},
Spec: registryv1alpha1.SchemaRegistrySpec{
URL:     "http://schema-registry.default.svc.cluster.local:8081",
Timeout: 30,
},
}
}

var _ = Describe("SchemaRegistry Webhook", func() {
var validator SchemaRegistryCustomValidator

BeforeEach(func() {
validator = SchemaRegistryCustomValidator{}
})

Context("ValidateCreate", func() {
It("Should accept a valid SchemaRegistry with no auth", func() {
_, err := validator.ValidateCreate(ctx, validSchemaRegistry())
Expect(err).NotTo(HaveOccurred())
})

It("Should reject when URL is empty", func() {
obj := validSchemaRegistry()
obj.Spec.URL = ""
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("url"))
})

It("Should reject when timeout is negative", func() {
obj := validSchemaRegistry()
obj.Spec.Timeout = -1
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("timeout"))
})

It("Should accept timeout of zero", func() {
obj := validSchemaRegistry()
obj.Spec.Timeout = 0
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).NotTo(HaveOccurred())
})

It("Should reject BASIC auth without basicAuth config", func() {
obj := validSchemaRegistry()
obj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type:      registryv1alpha1.AuthTypeBasic,
BasicAuth: nil,
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("basicAuth"))
})

It("Should accept BASIC auth with basicAuth config", func() {
obj := validSchemaRegistry()
obj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type: registryv1alpha1.AuthTypeBasic,
BasicAuth: &registryv1alpha1.BasicAuthConfig{
SecretRef: "creds-secret",
},
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).NotTo(HaveOccurred())
})

It("Should reject BEARER auth without bearerAuth config", func() {
obj := validSchemaRegistry()
obj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type:       registryv1alpha1.AuthTypeBearer,
BearerAuth: nil,
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("bearerAuth"))
})

It("Should accept BEARER auth with bearerAuth config", func() {
obj := validSchemaRegistry()
obj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type: registryv1alpha1.AuthTypeBearer,
BearerAuth: &registryv1alpha1.BearerAuthConfig{
SecretRef: "token-secret",
},
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).NotTo(HaveOccurred())
})

It("Should reject MTLS auth without mtls config", func() {
obj := validSchemaRegistry()
obj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type: registryv1alpha1.AuthTypeMTLS,
MTLS: nil,
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("mtls"))
})

It("Should accept MTLS auth with mtls config", func() {
obj := validSchemaRegistry()
obj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type: registryv1alpha1.AuthTypeMTLS,
MTLS: &registryv1alpha1.MTLSConfig{
CertSecretRef: "tls-secret",
},
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).NotTo(HaveOccurred())
})
})

Context("ValidateUpdate", func() {
It("Should accept a valid update changing URL", func() {
oldObj := validSchemaRegistry()
newObj := validSchemaRegistry()
newObj.Spec.URL = "http://schema-registry-v2.default.svc.cluster.local:8081"
_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
Expect(err).NotTo(HaveOccurred())
})

It("Should reject update that removes URL", func() {
oldObj := validSchemaRegistry()
newObj := validSchemaRegistry()
newObj.Spec.URL = ""
_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
Expect(err).To(HaveOccurred())
})

It("Should reject update setting BASIC auth without config", func() {
oldObj := validSchemaRegistry()
newObj := validSchemaRegistry()
newObj.Spec.Auth = &registryv1alpha1.AuthConfig{
Type: registryv1alpha1.AuthTypeBasic,
}
_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
Expect(err).To(HaveOccurred())
})
})

Context("ValidateDelete", func() {
It("Should always allow deletion", func() {
_, err := validator.ValidateDelete(ctx, validSchemaRegistry())
Expect(err).NotTo(HaveOccurred())
})
})
})
