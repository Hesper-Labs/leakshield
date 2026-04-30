# Deployment: Kubernetes

The repo ships a Helm chart at [`deploy/helm/leakshield/`](https://github.com/Hesper-Labs/leakshield/tree/main/deploy/helm/leakshield). It covers the gateway,
inspector, panel, an optional bundled Postgres + Redis (for "I just want to try this in my
cluster"), ingress, secrets, and HPAs.

## Prerequisites

- Kubernetes ≥ 1.27 (we use Postgres `UNIQUE NULLS NOT DISTINCT` which needs Postgres ≥ 15, but
  the cluster version itself only matters for stateful set guarantees and `topologySpreadConstraints`).
- `cert-manager` is recommended for TLS but not required.
- A KMS the gateway can talk to (Vault Transit, AWS / GCP / Azure KMS) is recommended for
  production. Local-file KEK works too — see [Encryption and KEK](Encryption-and-KEK).

## Install (try-it-in-your-cluster mode)

```bash
helm install leakshield deploy/helm/leakshield/ \
  --namespace leakshield --create-namespace \
  --set postgres.enabled=true \
  --set redis.enabled=true \
  --set kek.provider=local \
  --set kek.secret.create=true
```

That brings up everything in-cluster with bundled Postgres + Redis stateful sets and an
auto-generated KEK in a Secret. **This is fine for a demo, not for production data.**

## Production install

In production you typically:

- Use a managed Postgres (RDS, Cloud SQL, AlloyDB, Crunchy Data…) and pass the URL.
- Use a managed Redis (ElastiCache, Memorystore, Upstash…).
- Source the KEK from your KMS.
- Pin image tags to the semver release, not `latest`.

```yaml
# values.prod.yaml
image:
  pullPolicy: IfNotPresent
  tag: "0.1.0"

postgres:
  enabled: false
  externalDatabaseUrl: postgres://leakshield@db.example.com:5432/leakshield?sslmode=require

redis:
  enabled: false
  externalUrl: rediss://cache.example.com:6380/0

kek:
  provider: aws            # vault | aws | gcp | azure
  aws:
    region: eu-west-1
    keyId: arn:aws:kms:eu-west-1:123456789012:key/abcd-efgh

auth:
  jwtSecretRef:
    name: leakshield-secrets
    key: jwt_secret

ingress:
  enabled: true
  className: nginx
  host: leakshield.example.com
  tls:
    secretName: leakshield-tls
```

```bash
helm install leakshield deploy/helm/leakshield/ \
  --namespace leakshield --create-namespace \
  --values values.prod.yaml
```

## What the chart deploys

- **`gateway` Deployment + HorizontalPodAutoscaler.** Stateless; scales on CPU and
  `gateway_inflight_requests`. PodDisruptionBudget = 1 to keep at least one replica during
  rolling updates.
- **`inspector` Deployment + HPA.** Vertical first (bigger resources / GPU), then horizontal.
  GPU node selector is commented in `values.yaml` for clusters that have one.
- **`panel` Deployment.** Stateless. Behind the same ingress as the gateway, served at the root
  path; admin / proxy routes live under their respective prefixes on the gateway.
- **`postgres-statefulset` and `postgres-service`** (when `postgres.enabled=true`). For demos.
  Consider an operator (Zalando, CrunchyData) for production.
- **`redis-deployment` and `redis-service`** (when `redis.enabled=true`).
- **`secret`** that bundles KEK reference, JWT secret, DB URL, Redis URL.
- **`configmap`** for non-sensitive runtime knobs (log level, DLP defaults).
- **`networkpolicy`** locking egress: gateway can reach inspector, postgres, redis, and the
  upstream provider domains (configured allowlist).
- **`serviceaccount`** with the minimum IAM (cloud-specific annotation hooks for Workload
  Identity / IRSA).

## Operating

- **Rolling upgrades.** `helm upgrade leakshield deploy/helm/leakshield/ --values …` —
  Deployments roll one pod at a time, gateway drains on SIGTERM, no client-visible downtime as
  long as you have ≥ 2 replicas.
- **Migrations.** Run as a Helm post-install / pre-upgrade hook (TODO); for now invoke manually:
  `kubectl -n leakshield exec deploy/leakshield-gateway -- /usr/local/bin/leakshield migrate up`.
- **Backups.** Database dump + KEK rotation logs. The KEK lives in your KMS (or a Secret); the
  Helm chart never writes it to a PVC.
- **Observability.** All three pods emit Prometheus metrics on `:9090` and OpenTelemetry on
  whatever you point `LEAKSHIELD_OTLP_ENDPOINT` at. The chart exposes the metrics ports as
  ServiceMonitor targets when `monitoring.serviceMonitor.enabled=true`.

## Removing

```bash
helm uninstall leakshield --namespace leakshield
kubectl delete namespace leakshield
```

The bundled Postgres PVC is **not** deleted automatically. Drop it manually if you're sure:

```bash
kubectl delete pvc -n leakshield -l app.kubernetes.io/name=leakshield-postgres
```
