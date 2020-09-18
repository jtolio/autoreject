package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	goauth2 "google.golang.org/api/oauth2/v2"
	"gopkg.in/go-webhelp/whoauth2.v1"
	"gopkg.in/webhelp.v1/whcompat"
	"gopkg.in/webhelp.v1/whfatal"
	"gopkg.in/webhelp.v1/whlog"
	"gopkg.in/webhelp.v1/whmux"
	"gopkg.in/webhelp.v1/whsess"
)

var (
	// defined for real in a .gitignored secrets.go init function
	cookieSecret = []byte("secret")
	baseURL      = "https://pagename"
	oauthId      = "id"
	oauthSecret  = "secret"
	gcpProjectId = "gcp-project"
)

func listenAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return ":7070"
}

type IndexHandler struct {
	p *whoauth2.ProviderHandler
}

func (h *IndexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	t, err := h.p.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}
	w.Header().Set("Content-Type", "text/html")
	if t != nil {
		fmt.Fprintf(w, `
		  <p>Logged in | <a href="%s">Log out</a></p>
	  `, h.p.LogoutURL("/"))
	} else {
		fmt.Fprintf(w, `
		  <p><a href="%s">Log in</a> | Logged out</p>
	  `, h.p.LoginURL(r.RequestURI, false))
	}

	fmt.Fprintf(w, `<p><a href="/settings">Settings</a></p>`)
}

type SettingsHandler struct {
	p *whoauth2.ProviderHandler
}

func (h *SettingsHandler) getUserId(ctx context.Context) string {
	t, err := h.p.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}
	svc, err := goauth2.New(h.p.Provider().Config.Client(ctx, t))
	if err != nil {
		whfatal.Error(err)
	}
	ti, err := svc.Tokeninfo().Do()
	if err != nil {
		whfatal.Error(err)
	}
	if len(ti.UserId) == 0 {
		whfatal.Error(fmt.Errorf("invalid user id"))
	}
	return ti.UserId
}

func (h *SettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	t, err := h.p.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}

	srv, err := calendar.New(h.p.Provider().Config.Client(ctx, t))
	if err != nil {
		whfatal.Error(err)
	}
	events, err := srv.Events.List("primary").
		ShowDeleted(false).SingleEvents(true).
		TimeMin(time.Now().Format(time.RFC3339)).
		MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		whfatal.Error(err)
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<h3>Restricted</h3>`)
	fmt.Fprintf(w, `
		  <p>Logged in | <a href="%s">Log out</a></p>
		  <pre>`, h.p.LogoutURL("/"))

	for _, item := range events.Items {
		date := item.Start.DateTime
		if date == "" {
			date = item.Start.Date
		}
		fmt.Fprintf(w, "%v (%v)\n", item.Summary, date)
	}

	fmt.Fprintf(w, `</pre>`)
	fmt.Fprintf(w, `<pre>%s</pre>`, h.getUserId(ctx))
}

func main() {
	oauth := whoauth2.NewProviderHandler(
		whoauth2.Google(whoauth2.Config(oauth2.Config{
			ClientID:     oauthId,
			ClientSecret: oauthSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  baseURL + "/auth/_cb",
			Scopes: []string{
				goauth2.OpenIDScope,
				calendar.CalendarEventsScope,
			},
		})), "oauth-google", "/auth", whoauth2.RedirectURLs{})
	oauth.RequestOfflineTokens()

	panic(whlog.ListenAndServe(listenAddr(),
		whlog.LogRequests(whlog.Default,
			whlog.LogResponses(whlog.Default,
				whsess.HandlerWithStore(whsess.NewCookieStore(cookieSecret),
					whfatal.Catch(
						whmux.Dir{
							"": whmux.Exact(&IndexHandler{p: oauth}),
							"settings": oauth.LoginRequired(whmux.Exact(
								&SettingsHandler{p: oauth})),
							"auth": oauth}))))))
}
