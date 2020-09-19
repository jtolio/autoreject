package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"

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

func idGen() string {
	var data [32]byte
	_, err := rand.Read(data[:])
	if err != nil {
		whfatal.Error(err)
	}
	return hex.EncodeToString(data[:])
}

type Site struct {
	r *views.Renderer
}

func (s *Site) Token(ctx context.Context) *oauth2.Token {
	t, err := s.r.Provider.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}
	return t
}

func (s *Site) OAuth2Client(ctx context.Context) *http.Client {
	return s.r.Provider.Provider().Config.Client(ctx, s.Token(ctx))
}

func (s *Site) UserId(ctx context.Context) string {
	svc, err := goauth2.New(s.OAuth2Client(ctx))
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

func (s *Site) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	srv, err := calendar.New(s.OAuth2Client(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	var calendars []*calendar.CalendarListEntry
	err = srv.CalendarList.List().Context(ctx).Pages(ctx,
		func(l *calendar.CalendarList) error {
			for _, item := range l.Items {
				switch item.AccessRole {
				case "writer", "owner":
				default:
					continue
				}
				if item.Deleted || item.Hidden {
					continue
				}
				calendars = append(calendars, item)
			}
			return nil
		})
	if err != nil {
		whfatal.Error(err)
	}

	s.r.Render(w, r, "settings", map[string]interface{}{
		"Calendars": calendars,
	})
}

func (s *Site) Register(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	srv, err := calendar.New(s.OAuth2Client(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	_ = &calendar.Channel{
		Address: baseURL + "/event",
		Id:      idGen(),
		Type:    "web_hook",
	}
	_ = srv

	whfatal.Redirect("/settings")
}

func (s *Site) Unregister(w http.ResponseWriter, r *http.Request) {
	whfatal.Redirect("/settings")
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
				calendar.CalendarReadonlyScope,
				calendar.CalendarEventsScope,
			},
		})), "oauth-google", "/auth", whoauth2.RedirectURLs{})
	oauth.RequestOfflineTokens()
	rend := views.NewRenderer(oauth)
	site := &Site{r: rend}

	panic(whlog.ListenAndServe(listenAddr(),
		whlog.LogRequests(whlog.Default,
			whlog.LogResponses(whlog.Default,
				whsess.HandlerWithStore(whsess.NewCookieStore(cookieSecret),
					whfatal.Catch(
						whmux.Dir{
							"": whmux.Exact(rend.Simple("index")),
							"settings": oauth.LoginRequired(whmux.Exact(
								http.HandlerFunc(site.Settings))),
							"register": oauth.LoginRequired(whmux.Exact(
								http.HandlerFunc(site.Register))),
							"unregister": oauth.LoginRequired(whmux.Exact(
								http.HandlerFunc(site.Unregister))),
							"auth": oauth}))))))
}
