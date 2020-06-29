package diff

const defaultTemplateReport = `[
{{- $global := . -}}
{{- range $idx, $entry := . -}}
{
  "Api": "{{ $entry.API }}",
  "Kind": "{{ $entry.Kind }}",
  "Namespace": "{{ $entry.Namespace }}",
  "Name": "{{ $entry.Name }}",
  "Change": "{{ $entry.Change }}"
}{{ if not (last $idx  $global) }},{{ end }}
{{- end }}]`
