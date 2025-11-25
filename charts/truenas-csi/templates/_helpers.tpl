{{/*
Expand the name of the chart.
*/}}
{{- define "truenas-csi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "truenas-csi.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "truenas-csi.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "truenas-csi.labels" -}}
helm.sh/chart: {{ include "truenas-csi.chart" . }}
{{ include "truenas-csi.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "truenas-csi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "truenas-csi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Controller selector labels
*/}}
{{- define "truenas-csi.controllerSelectorLabels" -}}
{{ include "truenas-csi.selectorLabels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Node selector labels
*/}}
{{- define "truenas-csi.nodeSelectorLabels" -}}
{{ include "truenas-csi.selectorLabels" . }}
app.kubernetes.io/component: node
{{- end }}

{{/*
Create the name of the service account for the controller
*/}}
{{- define "truenas-csi.controllerServiceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-controller" (include "truenas-csi.fullname" .)) .Values.serviceAccount.controllerName }}
{{- else }}
{{- default "default" .Values.serviceAccount.controllerName }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account for the node
*/}}
{{- define "truenas-csi.nodeServiceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-node" (include "truenas-csi.fullname" .)) .Values.serviceAccount.nodeName }}
{{- else }}
{{- default "default" .Values.serviceAccount.nodeName }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret
*/}}
{{- define "truenas-csi.secretName" -}}
{{- if .Values.truenas.existingSecret }}
{{- .Values.truenas.existingSecret }}
{{- else }}
{{- include "truenas-csi.fullname" . }}
{{- end }}
{{- end }}

{{/*
Get the image tag
*/}}
{{- define "truenas-csi.imageTag" -}}
{{- .Values.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Get the CSI socket path
*/}}
{{- define "truenas-csi.socketPath" -}}
/csi/csi.sock
{{- end }}

{{/*
Get the CSI socket directory
*/}}
{{- define "truenas-csi.socketDir" -}}
/csi
{{- end }}

{{/*
Get the kubelet directory
*/}}
{{- define "truenas-csi.kubeletDir" -}}
/var/lib/kubelet
{{- end }}
