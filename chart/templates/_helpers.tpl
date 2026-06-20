{{/*
Expand the name of the chart.
*/}}
{{- define "nebari-catalog-pack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "nebari-catalog-pack.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart name and version.
*/}}
{{- define "nebari-catalog-pack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "nebari-catalog-pack.labels" -}}
helm.sh/chart: {{ include "nebari-catalog-pack.chart" . }}
{{ include "nebari-catalog-pack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "nebari-catalog-pack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nebari-catalog-pack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name.
*/}}
{{- define "nebari-catalog-pack.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "nebari-catalog-pack.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
NebariApp service name (defaults to the chart fullname).
*/}}
{{- define "nebari-catalog-pack.serviceName" -}}
{{- default (include "nebari-catalog-pack.fullname" .) .Values.nebariapp.service.name }}
{{- end }}
