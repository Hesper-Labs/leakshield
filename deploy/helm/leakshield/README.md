# LeakShield Helm chart

Install LeakShield (gateway + inspector + panel) on Kubernetes.

## Prerequisites

- Kubernetes >= 1.27
- Helm 3.12 or later
- Storage class for the bundled Postgres StatefulSet (or set `postgres.externalDatabaseUrl`)
- Optional: cert-manager for TLS, Vault or a cloud KMS for production KEK

## Install

```sh
helm repo add hesper-labs https://hesper-labs.github.io/charts   # placeholder; see Note below
helm install leakshield hesper-labs/leakshield \
  --namespace leakshield --create-namespace \
  --set ingress.enabled=true \
  --set ingress.host=leakshield.example.com
```

> Note: until the chart is published to a registry, install from a local checkout:
>
> ```sh
> helm install leakshield ./deploy/helm/leakshield \
>   --namespace leakshield --create-namespace
> ```

## Upgrade

```sh
helm upgrade leakshield hesper-labs/leakshield \
  --namespace leakshield \
  -f my-values.yaml
```

## Uninstall

```sh
helm uninstall leakshield --namespace leakshield
# Persistent volumes for the bundled Postgres are retained. Remove them
# manually if you want a clean slate:
kubectl -n leakshield delete pvc -l app.kubernetes.io/component=postgres
```

## Values reference

| Key | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/hesper-labs` | Image registry / namespace prefix |
| `image.tag` | `""` (chart's `appVersion`) | Pin a specific release |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | `[]` | Pull secrets for private registries |
| `gateway.replicaCount` | `2` | Initial replica count when HPA is off |
| `gateway.resources` | requests `200m` / `256Mi`, limits `2` / `1Gi` | Gateway container resources |
| `gateway.autoscaling.enabled` | `true` | Enable HorizontalPodAutoscaler |
| `gateway.autoscaling.minReplicas` | `2` | HPA minimum |
| `gateway.autoscaling.maxReplicas` | `10` | HPA maximum |
| `gateway.autoscaling.targetCPUUtilizationPercentage` | `70` | HPA target |
| `inspector.replicaCount` | `1` | Inspector replicas |
| `inspector.resources` | requests `1` / `1Gi`, limits `4` / `4Gi` | Inspector container resources |
| `inspector.backend` | `mock` | `mock` \| `ollama` \| `vllm` \| `llamacpp` \| `openai_compat` |
| `inspector.backendUrl` | `""` | URL of the model server when backend != `mock` |
| `inspector.judgeModel` | `qwen2.5:3b-instruct` | Default model name for judge / hybrid strategies |
| `panel.replicaCount` | `1` | Panel replicas |
| `panel.resources` | requests `100m` / `256Mi`, limits `1` / `512Mi` | Panel resources |
| `panel.publicGatewayUrl` | `""` (in-cluster service) | Public URL the panel uses to reach the gateway |
| `postgres.enabled` | `true` | Bundle a Postgres StatefulSet |
| `postgres.image.repository` | `postgres` | Postgres image |
| `postgres.image.tag` | `16-alpine` | Postgres image tag |
| `postgres.size` | `20Gi` | Persistent volume size |
| `postgres.externalDatabaseUrl` | `""` | Override to use an external Postgres |
| `postgres.auth.username` | `leakshield` | Bundled Postgres user |
| `postgres.auth.database` | `leakshield` | Bundled Postgres database name |
| `postgres.auth.password` | `""` | Inline password (NOT recommended); leave empty to auto-generate |
| `postgres.auth.existingSecret` | `""` | Reference an existing Secret with key `postgres-password` |
| `redis.enabled` | `true` | Bundle a Redis Deployment |
| `redis.externalUrl` | `""` | Override to use an external Redis |
| `kek.provider` | `local` | `local` \| `vault` \| `aws` \| `gcp` \| `azure` |
| `kek.secretName` | `""` | Reference an externally-managed KEK Secret |
| `kek.vault.address` | `https://vault.example.com:8200` | Vault address |
| `kek.vault.transitPath` | `transit/keys/leakshield` | Vault Transit path |
| `kek.vault.role` | `leakshield` | Vault role |
| `kek.vault.authPath` | `auth/kubernetes` | Vault auth path |
| `kek.aws.region` | `us-east-1` | AWS region |
| `kek.aws.keyId` | `alias/leakshield` | AWS KMS key id or alias |
| `kek.gcp.project` | `""` | GCP project |
| `kek.gcp.location` | `global` | GCP KMS location |
| `kek.gcp.keyRing` | `leakshield` | GCP KMS key ring |
| `kek.gcp.keyName` | `kek` | GCP KMS key name |
| `kek.azure.vaultUrl` | `https://leakshield.vault.azure.net` | Azure Key Vault URL |
| `kek.azure.keyName` | `leakshield-kek` | Azure key name |
| `auth.jwtSecret` | `""` | Inline JWT secret (NOT recommended); leave empty to auto-generate |
| `auth.existingSecret` | `""` | Reference an existing Secret with key `jwt-secret` |
| `auth.sso.enabled` | `false` | SSO toggle (placeholder) |
| `ingress.enabled` | `false` | Enable Ingress |
| `ingress.className` | `""` | Ingress class (e.g. `nginx`) |
| `ingress.host` | `leakshield.example.com` | Hostname |
| `ingress.tls` | `[]` | TLS configuration |
| `observability.otlp.endpoint` | `""` | OTLP endpoint |
| `observability.otlp.insecure` | `false` | Disable TLS to the collector |
| `observability.prometheus.enabled` | `true` | Expose Prometheus annotations on Services |
| `observability.prometheus.serviceMonitor.enabled` | `false` | Create a ServiceMonitor (kube-prometheus-stack) |
| `serviceAccount.create` | `true` | Create a ServiceAccount |
| `serviceAccount.name` | `""` | Override SA name |
| `serviceAccount.annotations` | `{}` | IRSA / Workload Identity annotations |
| `podSecurityContext` | non-root, fsGroup 65532 | PodSecurityContext |
| `containerSecurityContext` | drop ALL caps, RO root FS | Container security defaults |
| `nodeSelector` | `{}` | Pod scheduling |
| `tolerations` | `[]` | Pod scheduling |
| `affinity` | `{}` | Pod scheduling |
| `networkPolicy.enabled` | `false` | Lock down inter-pod traffic |
| `networkPolicy.allowedClientCidrs` | `[]` | CIDR blocks allowed to reach the gateway |

## Migrating from docker-compose

The Helm chart preserves the same env-var contract as `docker-compose.yml`,
so a docker-compose-based deployment maps onto the chart with no surprises:

| docker-compose | Helm value |
|---|---|
| `LEAKSHIELD_DATABASE_URL` | Computed from `postgres.*` or `postgres.externalDatabaseUrl` |
| `LEAKSHIELD_REDIS_URL` | Computed from `redis.*` or `redis.externalUrl` |
| `LEAKSHIELD_INSPECTOR_BACKEND` | `inspector.backend` |
| `LEAKSHIELD_INSPECTOR_BACKEND_URL` | `inspector.backendUrl` |
| `LEAKSHIELD_KEK_FILE` | Replaced by `kek.provider` + Secret reference |
| `AUTH_SECRET` | `auth.jwtSecret` (or `auth.existingSecret`) |

Steps to migrate:

1. Dump your Postgres data from the compose volume (`docker compose exec postgres pg_dumpall ...`).
2. Restore into a managed Postgres or the chart's bundled StatefulSet.
3. Copy the value of `LEAKSHIELD_KEK_FILE` (the raw 32 bytes) into a Kubernetes Secret named per
   `kek.secretName`, key `kek`. Or rotate to Vault / KMS now.
4. `helm install` with the values above.
5. Switch DNS / your ingress to the new endpoint.

## Hardening checklist

- [ ] `kek.provider` is `vault`, `aws`, `gcp`, or `azure`. The local provider is dev-only.
- [ ] `auth.existingSecret` references a Secret managed by ESO / Vault Agent / Sealed Secrets, not
      an inline value in `values.yaml`.
- [ ] `postgres.externalDatabaseUrl` points at a managed Postgres with backups + PITR.
- [ ] `ingress.tls` is configured and `cert-manager` (or equivalent) issues certs.
- [ ] `networkPolicy.enabled=true` and `allowedClientCidrs` lists only the office / VPC ranges
      that should reach the gateway.
- [ ] `podSecurityContext.runAsNonRoot=true` and `containerSecurityContext.readOnlyRootFilesystem=true`
      (chart defaults; do not relax).
- [ ] OTLP endpoint set so traces and metrics actually reach your observability stack.
