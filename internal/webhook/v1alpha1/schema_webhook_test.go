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

const validAvroSchema = `{"type":"record","name":"User","fields":[{"name":"id","type":"string"}]}`

func validSchema() *registryv1alpha1.Schema {
return &registryv1alpha1.Schema{
ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
Spec: registryv1alpha1.SchemaSpec{
Subject:    "users-value",
SchemaType: registryv1alpha1.SchemaTypeAvro,
Schema:     validAvroSchema,
RegistryRef: registryv1alpha1.SchemaRegistryRef{
Name: "my-registry",
},
},
}
}

var _ = Describe("Schema Webhook", func() {
var validator SchemaCustomValidator

BeforeEach(func() {
validator = SchemaCustomValidator{}
})

Context("ValidateCreate", func() {
It("Should accept a valid Schema", func() {
_, err := validator.ValidateCreate(ctx, validSchema())
Expect(err).NotTo(HaveOccurred())
})

It("Should reject when subject is empty", func() {
obj := validSchema()
obj.Spec.Subject = ""
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("subject"))
})

It("Should reject when schema content is empty", func() {
obj := validSchema()
obj.Spec.Schema = ""
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("schema"))
})

It("Should reject when registryRef.name is empty", func() {
obj := validSchema()
obj.Spec.RegistryRef.Name = ""
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("registryRef"))
})

It("Should reject AVRO schema with invalid JSON", func() {
obj := validSchema()
obj.Spec.SchemaType = registryv1alpha1.SchemaTypeAvro
obj.Spec.Schema = `not-valid-json`
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("valid JSON"))
})

It("Should reject JSON schema with invalid JSON", func() {
obj := validSchema()
obj.Spec.SchemaType = registryv1alpha1.SchemaTypeJSON
obj.Spec.Schema = `{broken`
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
})

It("Should accept PROTOBUF schema without JSON validation", func() {
obj := validSchema()
obj.Spec.SchemaType = registryv1alpha1.SchemaTypeProtobuf
obj.Spec.Schema = `syntax = "proto3"; message Foo { string id = 1; }`
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).NotTo(HaveOccurred())
})

It("Should reject reference with empty name", func() {
obj := validSchema()
obj.Spec.References = []registryv1alpha1.SchemaReference{
{Name: "", Subject: "other-value", Version: 1},
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("name"))
})

It("Should reject reference with version < 1", func() {
obj := validSchema()
obj.Spec.References = []registryv1alpha1.SchemaReference{
{Name: "ref", Subject: "other-value", Version: 0},
}
_, err := validator.ValidateCreate(ctx, obj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("version"))
})
})

Context("ValidateUpdate", func() {
It("Should accept a valid update", func() {
oldObj := validSchema()
newObj := validSchema()
newObj.Spec.Schema = `{"type":"record","name":"UserV2","fields":[{"name":"id","type":"string"},{"name":"email","type":"string"}]}`
_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
Expect(err).NotTo(HaveOccurred())
})

It("Should reject subject change", func() {
oldObj := validSchema()
newObj := validSchema()
newObj.Spec.Subject = "different-subject"
_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("immutable"))
})

It("Should reject schemaType change", func() {
oldObj := validSchema()
newObj := validSchema()
newObj.Spec.SchemaType = registryv1alpha1.SchemaTypeJSON
_, err := validator.ValidateUpdate(ctx, oldObj, newObj)
Expect(err).To(HaveOccurred())
Expect(err.Error()).To(ContainSubstring("immutable"))
})
})

Context("ValidateDelete", func() {
It("Should always allow deletion", func() {
_, err := validator.ValidateDelete(ctx, validSchema())
Expect(err).NotTo(HaveOccurred())
})
})
})
