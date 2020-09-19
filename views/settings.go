package views

var _ = T.MustParse(`{{template "header" .}}

<ul>
{{range .Values.Events.Items}}
<li>{{.Start.Date}} - {{.Summary}}</li>
{{end}}
</ul>

{{template "footer" .}}`)
