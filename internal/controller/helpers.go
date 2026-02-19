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
	"crypto/tls"
	"crypto/x509"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/honza/schema-strimzi-operator/api/v1alpha1"
	schemaclient "github.com/honza/schema-strimzi-operator/internal/client"
)

// loadAuthConfig reads authentication credentials from referenced Kubernetes Secrets
// and builds an AuthConfig for the Schema Registry HTTP client.
func loadAuthConfig(ctx context.Context, k8sClient client.Client, sr *registryv1alpha1.SchemaRegistry) (schemaclient.AuthConfig, error) {
	authConfig := schemaclient.AuthConfig{
		Type: "NONE",
	}

	if sr.Spec.Auth == nil {
		return authConfig, nil
	}

	authConfig.Type = string(sr.Spec.Auth.Type)

	switch sr.Spec.Auth.Type {
	case registryv1alpha1.AuthTypeBasic:
		if sr.Spec.Auth.BasicAuth == nil {
			return authConfig, fmt.Errorf("basicAuth config is required when type is BASIC")
		}

		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Name:      sr.Spec.Auth.BasicAuth.SecretRef,
			Namespace: sr.Namespace,
		}, secret); err != nil {
			return authConfig, fmt.Errorf("failed to get basic auth secret %q: %w", sr.Spec.Auth.BasicAuth.SecretRef, err)
		}

		authConfig.Username = string(secret.Data["username"])
		authConfig.Password = string(secret.Data["password"])

	case registryv1alpha1.AuthTypeBearer:
		if sr.Spec.Auth.BearerAuth == nil {
			return authConfig, fmt.Errorf("bearerAuth config is required when type is BEARER")
		}

		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Name:      sr.Spec.Auth.BearerAuth.SecretRef,
			Namespace: sr.Namespace,
		}, secret); err != nil {
			return authConfig, fmt.Errorf("failed to get bearer auth secret %q: %w", sr.Spec.Auth.BearerAuth.SecretRef, err)
		}

		authConfig.BearerToken = string(secret.Data["token"])

	case registryv1alpha1.AuthTypeMTLS:
		if sr.Spec.Auth.MTLS == nil {
			return authConfig, fmt.Errorf("mtls config is required when type is MTLS")
		}

		certSecret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Name:      sr.Spec.Auth.MTLS.CertSecretRef,
			Namespace: sr.Namespace,
		}, certSecret); err != nil {
			return authConfig, fmt.Errorf("failed to get client cert secret %q: %w", sr.Spec.Auth.MTLS.CertSecretRef, err)
		}

		cert, err := tls.X509KeyPair(certSecret.Data["tls.crt"], certSecret.Data["tls.key"])
		if err != nil {
			return authConfig, fmt.Errorf("failed to parse client certificate: %w", err)
		}

		authConfig.ClientCert = cert

		if sr.Spec.Auth.MTLS.CASecretRef != "" {
			caSecret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, client.ObjectKey{
				Name:      sr.Spec.Auth.MTLS.CASecretRef,
				Namespace: sr.Namespace,
			}, caSecret); err != nil {
				return authConfig, fmt.Errorf("failed to get CA cert secret %q: %w", sr.Spec.Auth.MTLS.CASecretRef, err)
			}

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caSecret.Data["ca.crt"])
			authConfig.CACert = caCertPool
		}
	}

	return authConfig, nil
}
