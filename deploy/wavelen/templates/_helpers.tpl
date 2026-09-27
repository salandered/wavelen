# https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
{{/* Identity labels. Every object uses them. */}}
{{- define "wavelen.labels" -}}
# The name of the application
app.kubernetes.io/name: {{ .Chart.Name }}
{{- /* A unique name identifying the instance of an application:
       helm upgrade --install <here is a release name> deploy/wavelen */}}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- /* The tool being used to manage the operation of an application.
       Helm stamps this */}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- /* The current version of the app (e.g. semver 1.0, revision hash).
       Same as 'image' in deployment and migration job. quote - we want a string. */}}
{{- define "wavelen.versionLabel" -}}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
{{- end }}

{{- /* Labels for the wavelen application objects: identity plus the app version.
       Postgres takes only wavelen.labels - it runs its own image,
       wavelen version would be misleading. */}}
{{- define "wavelen.appLabels" -}}
{{ include "wavelen.labels" . }}
{{ include "wavelen.versionLabel" . }}
{{- end }}

{{/* Selector labels, app workload only. 
	 Nothing that changes between releases (version, instance) is here.
     Postgres keeps its literal app: postgres. */}}
{{- define "wavelen.selectorLabels" -}}
app: wavelen
{{- end }}

{{- /* Traefik middlewares for the Ingress annotation, comma separated, applied in order.
       Empty when none is enabled.
       redirect-https goes first: a plain HTTP request gets its redirect without spending a token.

       https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress/#opt-traefik-ingress-kubernetes-iorouter-middlewares
       Old docs describe the format explicitly https://doc.traefik.io/traefik/v2.2/middlewares/overview/#provider-namespace:
	       <middleware-namespace>-<middleware-name>@kubernetescrd
       Latest doc version: https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/crd/http/middleware/ 
	   
	   If all enabled would be like "default-redirect-https@kubernetescrd,default-rate-limit@kubernetescrd" */}}
{{- define "wavelen.middlewares" -}}
{{- $m := list }}
{{- if .Values.redirect.enabled }}
{{- $m = append $m (printf "%s-redirect-https@kubernetescrd" .Release.Namespace) }}
{{- end }}
{{- if .Values.rateLimit.enabled }}
{{- $m = append $m (printf "%s-rate-limit@kubernetescrd" .Release.Namespace) }}
{{- end }}
{{- join "," $m }}
{{- end }}
