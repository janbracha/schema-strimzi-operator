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

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SchemaRegistryClient is an HTTP client for the Confluent Schema Registry API.
type SchemaRegistryClient struct {
	baseURL    string
	httpClient *http.Client
	auth       AuthConfig
}

// AuthConfig holds authentication configuration for connecting to Schema Registry.
type AuthConfig struct {
	// Type is the authentication type: NONE, BASIC, BEARER, MTLS
	Type string
	// Username for BASIC auth
	Username string
	// Password for BASIC auth
	Password string
	// BearerToken for BEARER auth
	BearerToken string
	// ClientCert for MTLS auth
	ClientCert tls.Certificate
	// CACert pool for MTLS auth
	CACert *x509.CertPool
}

// SchemaResponse represents the Schema Registry response for a registered schema.
type SchemaResponse struct {
	ID      int    `json:"id"`
	Version int    `json:"version"`
	Schema  string `json:"schema"`
}

// RegisterSchemaRequest is the request body sent to register a schema.
type RegisterSchemaRequest struct {
	Schema     string            `json:"schema"`
	SchemaType string            `json:"schemaType"`
	References []SchemaReference `json:"references,omitempty"`
}

// SchemaReference represents a reference to another schema subject.
type SchemaReference struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Version int    `json:"version"`
}

// NewClient creates a new SchemaRegistryClient.
func NewClient(baseURL string, auth AuthConfig, timeout time.Duration, insecureSkipVerify bool) (*SchemaRegistryClient, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec
	}

	if auth.Type == "MTLS" {
		tlsConfig.Certificates = []tls.Certificate{auth.ClientCert}
		if auth.CACert != nil {
			tlsConfig.RootCAs = auth.CACert
		}
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &SchemaRegistryClient{
		baseURL:    baseURL,
		httpClient: httpClient,
		auth:       auth,
	}, nil
}

// HealthCheck verifies connectivity to Schema Registry by listing subjects.
func (c *SchemaRegistryClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/subjects", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// RegisterSchema registers a schema under the given subject.
// If the schema already exists, the existing ID is returned (idempotent).
func (c *SchemaRegistryClient) RegisterSchema(ctx context.Context, subject string, request RegisterSchemaRequest) (*SchemaResponse, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.addAuth(req)
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to register schema: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("schema registration failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// The POST /subjects/<subject>/versions response only returns {"id": ...}.
	// We need a separate call to get the version number.
	var idResp struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(respBody, &idResp); err != nil {
		return nil, fmt.Errorf("failed to decode register response: %w", err)
	}

	// Fetch the latest version to get the version number for this schema ID.
	version, err := c.getLatestVersionForSubject(ctx, subject)
	if err != nil {
		// Non-fatal: return with ID but without version
		return &SchemaResponse{ID: idResp.ID}, nil
	}

	return &SchemaResponse{ID: idResp.ID, Version: version}, nil
}

// getLatestVersionForSubject retrieves the latest version number registered under subject.
func (c *SchemaRegistryClient) getLatestVersionForSubject(ctx context.Context, subject string) (int, error) {
	url := fmt.Sprintf("%s/subjects/%s/versions/latest", c.baseURL, subject)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("get latest version failed with status: %d", resp.StatusCode)
	}

	var result SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode latest version response: %w", err)
	}

	return result.Version, nil
}

// DeleteSubject deletes all versions of a subject from Schema Registry.
// Used during finalizer cleanup when a Schema CR is deleted.
func (c *SchemaRegistryClient) DeleteSubject(ctx context.Context, subject string) error {
	url := fmt.Sprintf("%s/subjects/%s", c.baseURL, subject)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete subject: %w", err)
	}
	defer resp.Body.Close()

	// 404 means already gone, which is fine for idempotent cleanup
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete subject failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SetCompatibility sets the compatibility level for the given subject.
func (c *SchemaRegistryClient) SetCompatibility(ctx context.Context, subject, level string) error {
	url := fmt.Sprintf("%s/config/%s", c.baseURL, subject)

	body := map[string]string{"compatibility": level}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	c.addAuth(req)
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set compatibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set compatibility failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// addAuth adds authentication headers to the request based on the configured auth type.
func (c *SchemaRegistryClient) addAuth(req *http.Request) {
	switch c.auth.Type {
	case "BASIC":
		req.SetBasicAuth(c.auth.Username, c.auth.Password)
	case "BEARER":
		req.Header.Set("Authorization", "Bearer "+c.auth.BearerToken)
	}
	// MTLS auth is handled via tls.Config in the transport layer
}
