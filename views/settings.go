package views

var _ = T.MustParse(`{{template "header" .}}

<p><a href="/register">Register</a></p>
<p><a href="/unregister">Unregister</a></p>

<ul>
{{range .Values.Calendars}}
<li>{{.Id}}: {{.Summary}} ({{.SummaryOverride}}) - {{.Description}}</li>
{{end}}
</ul>

{{template "footer" .}}`)
