{{- range $idx, $entry := . -}}
Resource name: {{ $entry.Name }}
{{- end -}}
