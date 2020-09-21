package views

var _ = T.MustParse(`{{template "header" .}}

<p>Google Calendar has a nice "Out of Office" event feature, where if you
schedule an "Out of Office" event, event invites during that time will be
automatically declined. Unfortunately, you can't make "Out of Office" events
be recurring.</p>
<p>This application watches your calendar and automatically rejects events
you are invited to if they conflict with special calendar events you create.</p>
<p>If you make sure the name of an event on your calendar includes the
below "Autoreject identifier" in the event name, then other events scheduled
during that time will be rejected with the provided "Autoreject reply."</p>

<form method="post">
<p>Autoreject identifier: <input type="text" name="autoreject_name" value="{{.Values.autoreject_name}}"></p>
<p>Autoreject reply: <input type="text" name="autoreject_reply" value="{{.Values.autoreject_reply}}"></p>
<p><input type="submit" value="Update"></p>
</form>

<p>You can register with the provided calendars individually below:</p>

<ul>
{{range .Values.Calendars}}
<li>{{if .Enabled}}
<form method="post" action="/unregister">
<input type="hidden" name="cal" value="{{.Id}}">
<input type="submit" value="Unregister calendar {{if (ne .SummaryOverride "")}}{{.SummaryOverride}}{{else}}{{.Summary}}{{end}}">
</form>
{{else}}
<form method="post" action="/register">
<input type="hidden" name="cal" value="{{.Id}}">
<input type="submit" value="Register calendar {{if (ne .SummaryOverride "")}}{{.SummaryOverride}}{{else}}{{.Summary}}{{end}}">
</form>
{{end}}
</li>
{{end}}
</ul>

{{template "footer" .}}`)
