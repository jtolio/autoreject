package views

var _ = T.MustParse(`{{template "header" .}}

<p><a href="/settings">Settings</a></p>

{{template "footer" .}}`)
