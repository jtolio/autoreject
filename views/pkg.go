package views

import (
	"context"
	"net/http"

	"gopkg.in/go-webhelp/whoauth2.v1"
	"gopkg.in/webhelp.v1/whcompat"
	"gopkg.in/webhelp.v1/whfatal"
	"gopkg.in/webhelp.v1/whtmpl"
)

var T = whtmpl.NewCollection()

type Page struct {
	LoggedIn  bool
	LoginURL  string
	LogoutURL string
	Req       *http.Request
	Ctx       context.Context
	Values    interface{}
}

type Renderer struct {
	Provider *whoauth2.ProviderHandler
}

func NewRenderer(provider *whoauth2.ProviderHandler) *Renderer {
	return &Renderer{Provider: provider}
}

func (rend *Renderer) Render(w http.ResponseWriter, r *http.Request,
	template string, values interface{}) {
	if values == nil {
		values = map[string]interface{}{}
	}

	ctx := whcompat.Context(r)

	t, err := rend.Provider.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}

	w.Header().Set("Content-Type", "text/html")
	T.Render(w, r, template, &Page{
		LoggedIn:  t != nil,
		LoginURL:  rend.Provider.LoginURL(r.RequestURI, true),
		LogoutURL: rend.Provider.LogoutURL("/"),
		Req:       r,
		Ctx:       ctx,
		Values:    values,
	})
}

func (rend *Renderer) Simple(template string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rend.Render(w, r, template, nil)
	})
}
