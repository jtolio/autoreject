package views

var _ = T.MustParse(`{{template "header" .}}

<p><a href="/register">Register</a></p>
<p><a href="/unregister">Unregister</a></p>

<ul>
{{range .Values.Calendars}}
<li>{{.Id}}: {{.Summary}} ({{.SummaryOverride}}) - {{.Description}}
{{if .Enabled}}
<form action="/unregister">
<input type="hidden" name="cal" value="{{.Id}}">
<input type="submit" value="Unregister">
</form>
{{else}}
<form action="/register">
<input type="hidden" name="cal" value="{{.Id}}">
<input type="submit" value="Register">
</form>
{{end}}
</li>
{{end}}
</ul>

{{template "footer" .}}`)
