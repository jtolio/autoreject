package views

var _ = T.MustParse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
	</head>
	<body>
		{{if .LoggedIn}}
			<p>Logged in | <a href="{{.LogoutURL}}">Log out</a></p>
		{{else}}
			<p>Logged out | <a href="{{.LoginURL}}">Log in</a></p>
		{{end}}
`)
