{{/*
Common helpers for the LeakShield chart.
*/}}

{{- define "leakshield.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "leakshield.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "leakshield.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "leakshield.labels" -}}
helm.sh/chart: {{ include "leakshield.chart" . }}
{{ include "leakshield.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: leakshield
{{- end -}}

{{- define "leakshield.selectorLabels" -}}
app.kubernetes.io/name: {{ include "leakshield.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Component-scoped helpers. Pass component name as the second argument when
calling these from templates: `{{ include "leakshield.componentLabels" (list . "gateway") }}`.
*/}}

{{- define "leakshield.componentName" -}}
{{- $ctx := index . 0 -}}
{{- $component := index . 1 -}}
{{- printf "%s-%s" (include "leakshield.fullname" $ctx) $component | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "leakshield.componentLabels" -}}
{{- $ctx := index . 0 -}}
{{- $component := index . 1 -}}
{{- include "leakshield.labels" $ctx }}
app.kubernetes.io/component: {{ $component }}
{{- end -}}

{{- define "leakshield.componentSelectorLabels" -}}
{{- $ctx := index . 0 -}}
{{- $component := index . 1 -}}
{{- include "leakshield.selectorLabels" $ctx }}
app.kubernetes.io/component: {{ $component }}
{{- end -}}

{{/*
Service account name.
*/}}
{{- define "leakshield.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "leakshield.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Image reference for a component. Falls back to global `image.repository`/`image.tag`.
Usage: `{{ include "leakshield.image" (dict "ctx" . "component" "gateway") }}`
*/}}
{{- define "leakshield.image" -}}
{{- $ctx := .ctx -}}
{{- $component := .component -}}
{{- $compValues := index $ctx.Values $component -}}
{{- $repo := default (printf "%s/leakshield-%s" $ctx.Values.image.repository $component) $compValues.image.repository -}}
{{- $tag := default (default $ctx.Chart.AppVersion $ctx.Values.image.tag) $compValues.image.tag -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}

{{/*
Database URL — either external override or in-cluster connection string for
the bundled Postgres StatefulSet.
*/}}
{{- define "leakshield.databaseUrl" -}}
{{- if .Values.postgres.externalDatabaseUrl -}}
{{- .Values.postgres.externalDatabaseUrl -}}
{{- else -}}
{{- $svc := printf "%s-postgres" (include "leakshield.fullname" .) -}}
{{- printf "postgres://%s:$(POSTGRES_PASSWORD)@%s:5432/%s?sslmode=disable" .Values.postgres.auth.username $svc .Values.postgres.auth.database -}}
{{- end -}}
{{- end -}}

{{/*
Redis URL — either external override or in-cluster service.
*/}}
{{- define "leakshield.redisUrl" -}}
{{- if .Values.redis.externalUrl -}}
{{- .Values.redis.externalUrl -}}
{{- else -}}
{{- $svc := printf "%s-redis" (include "leakshield.fullname" .) -}}
{{- printf "redis://%s:6379/0" $svc -}}
{{- end -}}
{{- end -}}

{{/*
Inspector address used by the gateway.
*/}}
{{- define "leakshield.inspectorAddr" -}}
{{- $svc := printf "%s-inspector" (include "leakshield.fullname" .) -}}
{{- printf "%s:%d" $svc (int .Values.inspector.service.port) -}}
{{- end -}}

{{/*
Secret name holding KEK / JWT / DB / Redis material.
*/}}
{{- define "leakshield.secretName" -}}
{{- if .Values.kek.secretName -}}
{{- .Values.kek.secretName -}}
{{- else -}}
{{- printf "%s-secrets" (include "leakshield.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "leakshield.configMapName" -}}
{{- printf "%s-config" (include "leakshield.fullname" .) -}}
{{- end -}}
