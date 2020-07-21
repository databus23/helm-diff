package diff

const defaultTemplateReport = `[
{{- $global := . -}}
{{- range $idx, $entry := . -}}
{
  "api": "{{ $entry.API }}",
  "kind": "{{ $entry.Kind }}",
  "namespace": "{{ $entry.Namespace }}",
  "name": "{{ $entry.Name }}",
  "change": "{{ $entry.Change }}"
}{{ if not (last $idx  $global) }},{{ end }}
{{- end }}]`
