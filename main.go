package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jtolio/autoreject/views"
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

func SimpleHandler(rend *views.Renderer, template string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rend.Render(w, r, template, nil)
	})
}

type SettingsHandler struct {
	r *views.Renderer
}

func (h *SettingsHandler) getUserId(ctx context.Context) string {
	t, err := h.r.Provider.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}
	svc, err := goauth2.New(h.r.Provider.Provider().Config.Client(ctx, t))
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
	t, err := h.r.Provider.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}

	srv, err := calendar.New(h.r.Provider.Provider().Config.Client(ctx, t))
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

	h.r.Render(w, r, "settings", map[string]interface{}{
		"Events": events,
	})
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
	rend := views.NewRenderer(oauth)

	panic(whlog.ListenAndServe(listenAddr(),
		whlog.LogRequests(whlog.Default,
			whlog.LogResponses(whlog.Default,
				whsess.HandlerWithStore(whsess.NewCookieStore(cookieSecret),
					whfatal.Catch(
						whmux.Dir{
							"": whmux.Exact(SimpleHandler(rend, "index")),
							"settings": oauth.LoginRequired(whmux.Exact(
								&SettingsHandler{r: rend})),
							"auth": oauth}))))))
}
