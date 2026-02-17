# Implementation Guide

Tento dokument popisuje, jak implementovat funkční logiku operátora.

## 1. Schema Registry Client

Nejdříve je potřeba vytvořit HTTP client pro komunikaci se Schema Registry API.

### Vytvoř adresář a soubory

```bash
mkdir -p internal/client
```

### internal/client/client.go

```go
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

type SchemaRegistryClient struct {
    baseURL    string
    httpClient *http.Client
    auth       AuthConfig
}

type AuthConfig struct {
    Type         string
    Username     string
    Password     string
    BearerToken  string
    ClientCert   tls.Certificate
    CACert       *x509.CertPool
}

type SchemaResponse struct {
    ID      int    `json:"id"`
    Version int    `json:"version"`
    Schema  string `json:"schema"`
}

type RegisterSchemaRequest struct {
    Schema     string            `json:"schema"`
    SchemaType string            `json:"schemaType"`
    References []SchemaReference `json:"references,omitempty"`
}

type SchemaReference struct {
    Name    string `json:"name"`
    Subject string `json:"subject"`
    Version int    `json:"version"`
}

func NewClient(baseURL string, auth AuthConfig, timeout time.Duration, insecureSkipVerify bool) (*SchemaRegistryClient, error) {
    tlsConfig := &tls.Config{
        InsecureSkipVerify: insecureSkipVerify,
    }

    // Configure mTLS if provided
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

// RegisterSchema registers a new schema or returns existing schema ID
func (c *SchemaRegistryClient) RegisterSchema(ctx context.Context, subject string, schema RegisterSchemaRequest) (*SchemaResponse, error) {
    url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject)
    
    body, err := json.Marshal(schema)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal schema: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("schema registration failed with status %d: %s", resp.StatusCode, string(body))
    }

    var result SchemaResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// GetSchema retrieves a schema by subject and version
func (c *SchemaRegistryClient) GetSchema(ctx context.Context, subject string, version int) (*SchemaResponse, error) {
    url := fmt.Sprintf("%s/subjects/%s/versions/%d", c.baseURL, subject, version)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    c.addAuth(req)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to get schema: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("get schema failed with status %d: %s", resp.StatusCode, string(body))
    }

    var result SchemaResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &result, nil
}

// SetCompatibility sets the compatibility level for a subject
func (c *SchemaRegistryClient) SetCompatibility(ctx context.Context, subject, level string) error {
    url := fmt.Sprintf("%s/config/%s", c.baseURL, subject)
    
    body := map[string]string{"compatibility": level}
    bodyBytes, err := json.Marshal(body)
    if err != nil {
        return err
    }

    req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
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

// HealthCheck verifies connectivity to Schema Registry
func (c *SchemaRegistryClient) HealthCheck(ctx context.Context) error {
    url := fmt.Sprintf("%s/subjects", c.baseURL)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

func (c *SchemaRegistryClient) addAuth(req *http.Request) {
    switch c.auth.Type {
    case "BASIC":
        req.SetBasicAuth(c.auth.Username, c.auth.Password)
    case "BEARER":
        req.Header.Set("Authorization", "Bearer "+c.auth.BearerToken)
    }
}
```

## 2. SchemaRegistry Controller

### internal/controller/schemaregistry_controller.go

Implementuj reconcile logiku:

```go
func (r *SchemaRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := logf.FromContext(ctx)

    // Fetch the SchemaRegistry instance
    var schemaRegistry registryv1alpha1.SchemaRegistry
    if err := r.Get(ctx, req.NamespacedName, &schemaRegistry); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Load auth configuration from secrets
    authConfig, err := r.loadAuthConfig(ctx, &schemaRegistry)
    if err != nil {
        log.Error(err, "Failed to load auth config")
        return ctrl.Result{}, err
    }

    // Create Schema Registry client
    timeout := time.Duration(schemaRegistry.Spec.Timeout) * time.Second
    srClient, err := schemaclient.NewClient(
        schemaRegistry.Spec.URL,
        authConfig,
        timeout,
        schemaRegistry.Spec.InsecureSkipVerify,
    )
    if err != nil {
        log.Error(err, "Failed to create Schema Registry client")
        return ctrl.Result{}, err
    }

    // Perform health check
    if err := srClient.HealthCheck(ctx); err != nil {
        log.Error(err, "Schema Registry health check failed")
        meta.SetStatusCondition(&schemaRegistry.Status.Conditions, metav1.Condition{
            Type:    "Ready",
            Status:  metav1.ConditionFalse,
            Reason:  "ConnectionFailed",
            Message: err.Error(),
        })
        schemaRegistry.Status.ConnectionStatus = "Unreachable"
    } else {
        meta.SetStatusCondition(&schemaRegistry.Status.Conditions, metav1.Condition{
            Type:    "Ready",
            Status:  metav1.ConditionTrue,
            Reason:  "Connected",
            Message: "Successfully connected to Schema Registry",
        })
        schemaRegistry.Status.ConnectionStatus = "Connected"
    }

    now := metav1.Now()
    schemaRegistry.Status.LastChecked = &now
    schemaRegistry.Status.ObservedGeneration = schemaRegistry.Generation

    if err := r.Status().Update(ctx, &schemaRegistry); err != nil {
        log.Error(err, "Failed to update status")
        return ctrl.Result{}, err
    }

    // Requeue for periodic health checks (every 5 minutes)
    return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *SchemaRegistryReconciler) loadAuthConfig(ctx context.Context, sr *registryv1alpha1.SchemaRegistry) (schemaclient.AuthConfig, error) {
    authConfig := schemaclient.AuthConfig{
        Type: string(sr.Spec.Auth.Type),
    }

    if sr.Spec.Auth == nil {
        return authConfig, nil
    }

    switch sr.Spec.Auth.Type {
    case registryv1alpha1.AuthTypeBasic:
        secret := &corev1.Secret{}
        if err := r.Get(ctx, client.ObjectKey{
            Name:      sr.Spec.Auth.BasicAuth.SecretRef,
            Namespace: sr.Namespace,
        }, secret); err != nil {
            return authConfig, fmt.Errorf("failed to get basic auth secret: %w", err)
        }
        authConfig.Username = string(secret.Data["username"])
        authConfig.Password = string(secret.Data["password"])

    case registryv1alpha1.AuthTypeBearer:
        secret := &corev1.Secret{}
        if err := r.Get(ctx, client.ObjectKey{
            Name:      sr.Spec.Auth.BearerAuth.SecretRef,
            Namespace: sr.Namespace,
        }, secret); err != nil {
            return authConfig, fmt.Errorf("failed to get bearer auth secret: %w", err)
        }
        authConfig.BearerToken = string(secret.Data["token"])

    case registryv1alpha1.AuthTypeMTLS:
        // Load client certificate
        certSecret := &corev1.Secret{}
        if err := r.Get(ctx, client.ObjectKey{
            Name:      sr.Spec.Auth.MTLS.CertSecretRef,
            Namespace: sr.Namespace,
        }, certSecret); err != nil {
            return authConfig, fmt.Errorf("failed to get client cert secret: %w", err)
        }

        cert, err := tls.X509KeyPair(certSecret.Data["tls.crt"], certSecret.Data["tls.key"])
        if err != nil {
            return authConfig, fmt.Errorf("failed to load client certificate: %w", err)
        }
        authConfig.ClientCert = cert

        // Load CA certificate if provided
        if sr.Spec.Auth.MTLS.CASecretRef != "" {
            caSecret := &corev1.Secret{}
            if err := r.Get(ctx, client.ObjectKey{
                Name:      sr.Spec.Auth.MTLS.CASecretRef,
                Namespace: sr.Namespace,
            }, caSecret); err != nil {
                return authConfig, fmt.Errorf("failed to get CA cert secret: %w", err)
            }

            caCertPool := x509.NewCertPool()
            caCertPool.AppendCertsFromPEM(caSecret.Data["ca.crt"])
            authConfig.CACert = caCertPool
        }
    }

    return authConfig, nil
}
```

## 3. Schema Controller

### internal/controller/schema_controller.go

Implementuj reconcile logiku:

```go
func (r *SchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := logf.FromContext(ctx)

    // Fetch the Schema instance
    var schema registryv1alpha1.Schema
    if err := r.Get(ctx, req.NamespacedName, &schema); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Fetch the referenced SchemaRegistry
    registryNamespace := schema.Spec.RegistryRef.Namespace
    if registryNamespace == "" {
        registryNamespace = schema.Namespace
    }

    var schemaRegistry registryv1alpha1.SchemaRegistry
    if err := r.Get(ctx, client.ObjectKey{
        Name:      schema.Spec.RegistryRef.Name,
        Namespace: registryNamespace,
    }, &schemaRegistry); err != nil {
        log.Error(err, "Failed to fetch SchemaRegistry")
        return ctrl.Result{}, err
    }

    // Load auth config and create client
    authConfig, err := r.loadAuthConfig(ctx, &schemaRegistry)
    if err != nil {
        log.Error(err, "Failed to load auth config")
        return ctrl.Result{}, err
    }

    timeout := time.Duration(schemaRegistry.Spec.Timeout) * time.Second
    srClient, err := schemaclient.NewClient(
        schemaRegistry.Spec.URL,
        authConfig,
        timeout,
        schemaRegistry.Spec.InsecureSkipVerify,
    )
    if err != nil {
        log.Error(err, "Failed to create Schema Registry client")
        return ctrl.Result{}, err
    }

    // Register the schema
    registerReq := schemaclient.RegisterSchemaRequest{
        Schema:     schema.Spec.Schema,
        SchemaType: string(schema.Spec.SchemaType),
        References: convertReferences(schema.Spec.References),
    }

    resp, err := srClient.RegisterSchema(ctx, schema.Spec.Subject, registerReq)
    if err != nil {
        log.Error(err, "Failed to register schema")
        meta.SetStatusCondition(&schema.Status.Conditions, metav1.Condition{
            Type:    "Ready",
            Status:  metav1.ConditionFalse,
            Reason:  "RegistrationFailed",
            Message: err.Error(),
        })
        if err := r.Status().Update(ctx, &schema); err != nil {
            return ctrl.Result{}, err
        }
        return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
    }

    // Set compatibility level if specified
    if schema.Spec.CompatibilityLevel != "" {
        if err := srClient.SetCompatibility(ctx, schema.Spec.Subject, schema.Spec.CompatibilityLevel); err != nil {
            log.Error(err, "Failed to set compatibility level")
        }
    }

    // Update status
    schema.Status.SchemaID = &resp.ID
    schema.Status.Version = &resp.Version
    now := metav1.Now()
    schema.Status.RegisteredAt = &now
    schema.Status.ObservedGeneration = schema.Generation

    meta.SetStatusCondition(&schema.Status.Conditions, metav1.Condition{
        Type:    "Ready",
        Status:  metav1.ConditionTrue,
        Reason:  "Registered",
        Message: fmt.Sprintf("Schema registered with ID %d, version %d", resp.ID, resp.Version),
    })

    if err := r.Status().Update(ctx, &schema); err != nil {
        log.Error(err, "Failed to update status")
        return ctrl.Result{}, err
    }

    log.Info("Schema successfully registered", "schemaID", resp.ID, "version", resp.Version)
    return ctrl.Result{}, nil
}

func convertReferences(refs []registryv1alpha1.SchemaReference) []schemaclient.SchemaReference {
    result := make([]schemaclient.SchemaReference, len(refs))
    for i, ref := range refs {
        result[i] = schemaclient.SchemaReference{
            Name:    ref.Name,
            Subject: ref.Subject,
            Version: ref.Version,
        }
    }
    return result
}
```

## 4. RBAC Permissions

V controller souborech přidej RBAC markery pro přístup ke Secrets:

```go
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
```

Potom vygeneruj znovu RBAC:

```bash
make manifests
```

## 5. Testování

### Unit testy

Otestuj client v izolaci s mock HTTP serverem.

### Integration testy

Použij testcontainers pro spuštění Schema Registry:

```go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupSchemaRegistry(t *testing.T) string {
    req := testcontainers.ContainerRequest{
        Image:        "confluentinc/cp-schema-registry:7.5.0",
        ExposedPorts: []string{"8081/tcp"},
        Env: map[string]string{
            "SCHEMA_REGISTRY_HOST_NAME":                   "schema-registry",
            "SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS": "PLAINTEXT://mock:9092",
        },
        WaitingFor: wait.ForHTTP("/subjects").WithPort("8081/tcp"),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)

    host, err := container.Host(ctx)
    require.NoError(t, err)

    port, err := container.MappedPort(ctx, "8081")
    require.NoError(t, err)

    return fmt.Sprintf("http://%s:%s", host, port.Port())
}
```

## 6. Next Steps

1. Implementuj client podle návodu výše
2. Implementuj controller logiku
3. Přidej finalizery pro cleanup při smazání Schema
4. Přidej unit a integration testy
5. Přidej webhooks pro validaci
6. Deploy do clusteru a otestuj e2e
7. Přidej metriky a monitoring
8. Dokumentuj API

## Reference

- [Schema Registry API Docs](https://docs.confluent.io/platform/current/schema-registry/develop/api.html)
- [Controller Runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubebuilder Book](https://book.kubebuilder.io/)
