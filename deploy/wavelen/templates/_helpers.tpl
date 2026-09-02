# https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
{{/* Object labels. */}}
{{- define "wavelen.labels" -}}
# The name of the application
app.kubernetes.io/name: {{ .Chart.Name }}
# A unique name identifying the instance of an application
# helm upgrade --install <here is a release name> deploy/wavelen
app.kubernetes.io/instance: {{ .Release.Name }}

{{- /* The current version of the app (e.g. semver 1.0, revision hash). 
       Same as 'image' in deployment and migration job. quote - we want a string. */}}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
{{- /* The tool being used to manage the operation of an application
       Helm stamps this. */}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/* Selector labels, app workload only. 
	 Nothing that changes between releases (version, instance) is here.
     Postgres keeps its literal app: postgres. */}}
{{- define "wavelen.selectorLabels" -}}
app: wavelen
{{- end }}
