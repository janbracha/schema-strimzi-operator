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

package client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/honza/schema-strimzi-operator/internal/client"
)

const (
	testSubject    = "users-value"
	testSchemaJSON = `{"type":"record","name":"User","fields":[{"name":"id","type":"string"}]}`
)

func newTestClient(t *testing.T, srv *httptest.Server, auth client.AuthConfig) *client.SchemaRegistryClient {
	t.Helper()
	c, err := client.NewClient(srv.URL, auth, 5*time.Second, false)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestHealthCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subjects" || r.Method != http.MethodGet {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestHealthCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.HealthCheck(context.Background()); err == nil {
		t.Error("expected error for 503, got nil")
	}
}

func TestHealthCheck_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := c.HealthCheck(ctx); err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
}

func TestRegisterSchema_OK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/subjects/"+testSubject+"/versions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":42}`))
	})
	mux.HandleFunc("/subjects/"+testSubject+"/versions/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := client.SchemaResponse{ID: 42, Version: 3, Schema: testSchemaJSON}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	resp, err := c.RegisterSchema(context.Background(), testSubject, client.RegisterSchemaRequest{
		Schema:     testSchemaJSON,
		SchemaType: "AVRO",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 42 {
		t.Errorf("expected ID 42, got %d", resp.ID)
	}
	if resp.Version != 3 {
		t.Errorf("expected version 3, got %d", resp.Version)
	}
}

func TestRegisterSchema_ContentTypeHeader(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/versions") && r.Method == http.MethodPost {
			gotContentType = r.Header.Get("Content-Type")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.SchemaResponse{ID: 1, Version: 1})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	_, _ = c.RegisterSchema(context.Background(), testSubject, client.RegisterSchemaRequest{
		Schema:     testSchemaJSON,
		SchemaType: "AVRO",
	})

	expected := "application/vnd.schemaregistry.v1+json"
	if gotContentType != expected {
		t.Errorf("expected Content-Type %q, got %q", expected, gotContentType)
	}
}

func TestRegisterSchema_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error_code":42201,"message":"Invalid schema"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	_, err := c.RegisterSchema(context.Background(), testSubject, client.RegisterSchemaRequest{
		Schema:     "invalid",
		SchemaType: "AVRO",
	})
	if err == nil {
		t.Error("expected error for 422, got nil")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error should mention status 422, got: %v", err)
	}
}

func TestRegisterSchema_WithReferences(t *testing.T) {
	var gotBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/subjects/"+testSubject+"/versions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":10}`))
	})
	mux.HandleFunc("/subjects/"+testSubject+"/versions/latest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.SchemaResponse{ID: 10, Version: 1})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	_, err := c.RegisterSchema(context.Background(), testSubject, client.RegisterSchemaRequest{
		Schema:     testSchemaJSON,
		SchemaType: "AVRO",
		References: []client.SchemaReference{
			{Name: "address", Subject: "address-value", Version: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	refs, ok := gotBody["references"].([]interface{})
	if !ok || len(refs) != 1 {
		t.Errorf("expected 1 reference in request body, got: %v", gotBody["references"])
	}
}

func TestDeleteSubject_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[1,2,3]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.DeleteSubject(context.Background(), testSubject); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteSubject_NotFound_Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error_code":40401,"message":"Subject not found"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.DeleteSubject(context.Background(), testSubject); err != nil {
		t.Errorf("expected nil for 404 (idempotent), got: %v", err)
	}
}

func TestDeleteSubject_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.DeleteSubject(context.Background(), testSubject); err == nil {
		t.Error("expected error for 500, got nil")
	}
}

func TestSetCompatibility_OK(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"compatibility":"BACKWARD"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.SetCompatibility(context.Background(), testSubject, "BACKWARD"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["compatibility"] != "BACKWARD" {
		t.Errorf("expected compatibility BACKWARD in request, got: %v", gotBody)
	}
}

func TestSetCompatibility_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error_code":42203,"message":"Invalid compatibility level"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	if err := c.SetCompatibility(context.Background(), testSubject, "INVALID"); err == nil {
		t.Error("expected error for 422, got nil")
	}
}

func TestAuth_Basic(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{
		Type:     "BASIC",
		Username: "alice",
		Password: "secret",
	})
	_ = c.HealthCheck(context.Background())

	if !strings.HasPrefix(gotAuthHeader, "Basic ") {
		t.Fatalf("expected Basic auth header, got: %q", gotAuthHeader)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(gotAuthHeader, "Basic "))
	if string(decoded) != "alice:secret" {
		t.Errorf("expected alice:secret, got: %q", string(decoded))
	}
}

func TestAuth_Bearer(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{
		Type:        "BEARER",
		BearerToken: "my-token-xyz",
	})
	_ = c.HealthCheck(context.Background())

	expected := "Bearer my-token-xyz"
	if gotAuthHeader != expected {
		t.Errorf("expected %q, got %q", expected, gotAuthHeader)
	}
}

func TestAuth_None_NoAuthHeader(t *testing.T) {
	var gotAuthHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv, client.AuthConfig{Type: "NONE"})
	_ = c.HealthCheck(context.Background())

	if gotAuthHeader != "" {
		t.Errorf("expected no Authorization header for NONE auth, got: %q", gotAuthHeader)
	}
}
