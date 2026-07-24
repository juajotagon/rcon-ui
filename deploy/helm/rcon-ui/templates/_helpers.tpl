{{- define "rcon-ui.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "rcon-ui.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "rcon-ui.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "rcon-ui.labels" -}}
app.kubernetes.io/name: {{ include "rcon-ui.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "rcon-ui.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rcon-ui.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Fail closed on authentication.

The container binds to 0.0.0.0, so anything that can reach the Service can drive
every configured game server. A chart that rendered without a token would hand
out RCON access to the whole cluster network, so this is a hard error rather
than a warning nobody reads.
*/}}
{{- define "rcon-ui.validate" -}}
{{- if and (not .Values.auth.token) (not .Values.auth.existingSecret) -}}
{{- fail "\n\nrcon-ui: authentication is required.\n\nThe daemon listens on all interfaces inside the container, so anyone who can\nreach the Service can control every game server you configure.\n\nSet one of:\n  --set auth.token=<a long random string>\n  --set auth.existingSecret=<name of a Secret you manage>\n" -}}
{{- end -}}
{{- if gt (int .Values.replicaCount) 1 -}}
{{- fail "\n\nrcon-ui: replicaCount must be 1.\n\nState is held in SQLite on a ReadWriteOnce volume. A second replica would share\nthat file and corrupt it. Scaling out needs a different store, not more pods.\n" -}}
{{- end -}}
{{- end -}}
