package views

var _ = T.MustParse(`{{template "header" .}}

<form method="post">
<p>Autoreject identifier: <input type="text" name="autoreject_name" value="{{.Values.autoreject_name}}"></p>
<p>Autoreject reply: <input type="text" name="autoreject_reply" value="{{.Values.autoreject_reply}}"></p>
<p><input type="submit" value="Update"></p>
</form>

<ul>
{{range .Values.Calendars}}
<li>{{.Id}}: {{.Summary}} ({{.SummaryOverride}}) - {{.Description}}
{{if .Enabled}}
<form method="post" action="/unregister">
<input type="hidden" name="cal" value="{{.Id}}">
<input type="submit" value="Unregister">
</form>
{{else}}
<form method="post" action="/register">
<input type="hidden" name="cal" value="{{.Id}}">
<input type="submit" value="Register">
</form>
{{end}}
</li>
{{end}}
</ul>

{{template "footer" .}}`)
