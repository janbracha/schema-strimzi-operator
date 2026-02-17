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

- **API definice** (`api/v1alpha1/`): Go struktury definující CRDs
- **Controllers** (`internal/controller/`): Reconciliation logika pro synchronizaci s Schema Registry
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

### Deployment do clusteru

```bash
# Build a push Docker image
make docker-build docker-push IMG=<your-registry>/schema-strimzi-operator:tag

# Deploy do clusteru
make deploy IMG=<your-registry>/schema-strimzi-operator:tag

# Undeploy
make undeploy
```

## Implementace - Další kroky

Aktuální stav je *scaffold* - základní struktura projektu. Pro plnou funkčnost je potřeba implementovat:

### 1. Schema Registry Client

V `internal/client/` vytvořit HTTP client pro komunikaci se Schema Registry:
- Registrace nových schémat
- Aktualizace existujících schémat
- Získání informací o schématu (ID, verze)
- Nastavení compatibility levelu
- Podpora autentizace (Basic, Bearer, mTLS)

**Doporučené knihovny:**
- `github.com/riferrei/srclient` - Go client pro Confluent Schema Registry
- Nebo vlastní implementace nad `net/http`

### 2. Controller logika

#### SchemaRegistryController (`internal/controller/schemaregistry_controller.go`)
- Validace připojení k Schema Registry
- Načtení credentials ze Secrets
- Pravidelné health checks
- Update status (connectionStatus, conditions)

#### SchemaController (`internal/controller/schema_controller.go`)
- Reconcile loop:
  1. Načíst Schema resource
  2. Získat SchemaRegistry configuraci
  3. Připojit se k Schema Registry
  4. Registrovat/aktualizovat schéma
  5. Aktualizovat status (schemaId, version)
- Finalizery pro cleanup
- Handling error stavů a retries
- Event recording

### 3. Testování

- Unit testy pro client (`internal/client/`)
- Controller testy s fake clients
- Integration testy s testcontainers (Schema Registry in Docker)
- E2E testy v reálném clusteru

### 4. Další vylepšení

- **Webhooks**: Validace schémat před uložením (admission webhook)
- **Metrics**: Prometheus metriky (počet registrovaných schémat, chyby, latence)
- **Subject management**: CRD pro správu subjects (delete, compatibility level)
- **Schema evolution**: Automatická migrace schémat
- **Integration s Strimzi**: Automatické vytváření schémat pro KafkaTopics

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
├── internal/controller/        # Controller implementace
│   ├── schema_controller.go
│   └── schemaregistry_controller.go
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

