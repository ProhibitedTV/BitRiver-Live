{{- define "bitriver-live.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "bitriver-live.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "bitriver-live.labels" -}}
app.kubernetes.io/name: {{ include "bitriver-live.name" . }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "bitriver-live.selectorLabels" -}}
app.kubernetes.io/name: {{ include "bitriver-live.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "bitriver-live.postgresDsn" -}}
{{- if .Values.secrets.BITRIVER_LIVE_POSTGRES_DSN }}
{{- .Values.secrets.BITRIVER_LIVE_POSTGRES_DSN -}}
{{- else -}}
{{- printf "postgres://%s:%s@%s-postgres:%d/%s?sslmode=disable" .Values.secrets.BITRIVER_POSTGRES_USER .Values.secrets.BITRIVER_POSTGRES_PASSWORD (include "bitriver-live.fullname" .) (.Values.service.postgres.port | int) .Values.env.BITRIVER_POSTGRES_DB -}}
{{- end -}}
{{- end -}}
