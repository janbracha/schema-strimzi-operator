# Schema Registry Operator for Strimzi

Kubernetes operator pro správu schémat v Schema Registry s integrací pro Strimzi Kafka.

## Přehled

Tento operátor poskytuje Kubernetes-native způsob správy schémat v Schema Registry. Umožňuje deklarativní registraci a orchestraci schémat pomocí Custom Resource Definitions (CRDs).

## Custom Resources

### SchemaRegistry

Reprezentuje Schema Registry endpoint s konfigurací připojení a autentizací.

**Příklad:**
```yaml
apiVersion: registry.strimzi.io/v1alpha1
kind: SchemaRegistry
metadata:
  name: my-schema-registry
  namespace: kafka
spec:
  url: "http://schema-registry.kafka.svc.cluster.local:8081"
  auth:
    type: BASIC
    basicAuth:
      secretRef: schema-registry-credentials
  timeout: 30
  insecureSkipVerify: false
```

**Podporované typy autentizace:**
- `NONE` - bez autentizace
- `BASIC` - Basic Auth (username/password)
- `BEARER` - Bearer token
- `MTLS` - Mutual TLS

### Schema

Reprezentuje jednotlivé schéma registrované v Schema Registry.

**Příklad AVRO schématu:**
```yaml
apiVersion: registry.strimzi.io/v1alpha1
kind: Schema
metadata:
  name: user-schema
  namespace: kafka
spec:
  subject: "users-value"
  schemaType: AVRO
  schema: |
    {
      "type": "record",
      "name": "User",
      "namespace": "com.example",
      "fields": [
        {"name": "id", "type": "string"},
        {"name": "name", "type": "string"},
        {"name": "email", "type": "string"}
      ]
    }
  registryRef:
    name: my-schema-registry
  compatibilityLevel: BACKWARD
```

**Podporované typy schémat:**
- `AVRO` - Apache Avro
- `JSON` - JSON Schema
- `PROTOBUF` - Protocol Buffers

**Podporované compatibility levels:**
- `BACKWARD` - Nová verze může číst data napsaná starší verzí
- `BACKWARD_TRANSITIVE` - Backward kompatibilita se všemi předchozími verzemi
- `FORWARD` - Stará verze může číst data napsaná novou verzí
- `FORWARD_TRANSITIVE` - Forward kompatibilita se všemi předchozími verzemi
- `FULL` - Kombinace BACKWARD a FORWARD
- `FULL_TRANSITIVE` - Full kompatibilita se všemi předchozími verzemi
- `NONE` - Bez kontroly kompatibility

## Architektura

Operátor je postaven na Kubebuilder frameworku a obsahuje:

- **API definice** (`api/v1alpha1/`): Go struktury definující CRDs pro `SchemaRegistry` a `Schema`
- **HTTP Client** (`internal/client/`): Implementace Confluent Schema Registry API (health check, registrace schémat, kompatibilita, mazání)
- **Controllers** (`internal/controller/`): Reconciliation logika pro synchronizaci s Schema Registry, watches na Secrets a SchemaRegistry změny
- **Webhooks** (`internal/webhook/v1alpha1/`): Validační admission webhooks pro obě CRD
- **Config** (`config/`): Kubernetes manifesty (CRDs, RBAC, deployment)

## Development

### Prerequisites

- Go 1.22+ (doporučeno 1.23+)
- Kubebuilder v4.12+
- kubectl
- Přístup ke Kubernetes clusteru (pro testování)

### Build

```bash
# Vygenerovat CRD manifesty
make manifests

# Vygenerovat Go kód (DeepCopy metody)
make generate

# Build binary
make build

# Run testy
make test
```

### Локální vývoj

```bash
# Nainstalovat CRDs do clusteru
make install

# Spustit controller lokálně (mimo cluster)
make run

# Uninstall CRDs
make uninstall
```

## Deployment do clusteru

```bash
# Build a push Docker image
make docker-build docker-push IMG=<your-registry>/schema-strimzi-operator:v0.1.0

# Deploy do clusteru
make deploy IMG=<your-registry>/schema-strimzi-operator:v0.1.0

# Undeploy
make undeploy
```

### Distribuce přes YAML bundle

```bash
# Vygenerovat dist/install.yaml (obsahuje CRDs, RBAC a Deployment)
make build-installer IMG=<your-registry>/schema-strimzi-operator:v0.1.0

# Instalace v clusteru
kubectl apply -f dist/install.yaml
```

## Struktura projektu

```
.
├── api/v1alpha1/              # CRD API definice
│   ├── schema_types.go        # Schema CRD
│   └── schemaregistry_types.go # SchemaRegistry CRD
├── cmd/                        # Main aplikace
├── config/                     # Kubernetes manifesty
│   ├── crd/bases/             # Vygenerované CRDs
│   ├── rbac/                  # Role-based access control
│   ├── manager/               # Deployment konfigurace
│   └── samples/               # Ukázkové CR manifesty
├── internal/
│   ├── client/                # HTTP client pro Schema Registry API
│   ├── controller/            # Controller reconciliation logika
│   └── webhook/v1alpha1/      # Validační admission webhooks
└── test/                       # E2E testy
```

## Příklady použití

Ukázkové manifesty najdeš v `config/samples/`:

```bash
# Vytvořit SchemaRegistry
kubectl apply -f config/samples/registry_v1alpha1_schemaregistry.yaml

# Vytvořit Schema
kubectl apply -f config/samples/registry_v1alpha1_schema.yaml

# Zobrazit status
kubectl get schemaregistries
kubectl get schemas
kubectl describe schema user-schema
```

## Contributing

1. Fork repository
2. Vytvoř feature branch (`git checkout -b feature/amazing-feature`)
3. Commit změny (`git commit -m 'Add amazing feature'`)
4. Push do branch (`git push origin feature/amazing-feature`)
5. Otevři Pull Request

## License

Apache License 2.0 - viz [LICENSE](LICENSE) file.

## Reference

- [Kubebuilder Documentation](https://book.kubebuilder.io/)
- [Confluent Schema Registry API](https://docs.confluent.io/platform/current/schema-registry/develop/api.html)
- [Strimzi Kafka Operator](https://strimzi.io/)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

