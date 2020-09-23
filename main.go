package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sort"

	"github.com/jtolio/autoreject/views"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	goauth2 "google.golang.org/api/oauth2/v2"
	"gopkg.in/go-webhelp/whoauth2.v1"
	"gopkg.in/webhelp.v1"
	"gopkg.in/webhelp.v1/whcache"
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

	OAuth2Token  = webhelp.GenSym()
	OAuth2Client = webhelp.GenSym()
	UserId       = webhelp.GenSym()
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
	r  *views.Renderer
	db *DB
}

func (s *Site) OAuth2Token(ctx context.Context) *oauth2.Token {
	if tok, ok := whcache.Get(ctx, OAuth2Token).(*oauth2.Token); ok {
		return tok
	}
	tok, err := s.r.Provider.Token(ctx)
	if err != nil {
		whfatal.Error(err)
	}
	whcache.Set(ctx, OAuth2Token, tok)
	return tok
}

func (s *Site) OAuth2Client(ctx context.Context) *http.Client {
	if cli, ok := whcache.Get(ctx, OAuth2Client).(*http.Client); ok {
		return cli
	}
	cli := s.r.Provider.Provider().Config.Client(ctx, s.OAuth2Token(ctx))
	whcache.Set(ctx, OAuth2Client, cli)
	return cli
}

func (s *Site) UserId(ctx context.Context) string {
	if id, ok := whcache.Get(ctx, UserId).(string); ok {
		return id
	}

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
	whcache.Set(ctx, UserId, ti.UserId)
	return ti.UserId
}

func (s *Site) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)

	for _, field := range []string{"autoreject_name", "autoreject_reply"} {
		if val := r.FormValue(field); val != "" {
			err := s.db.SetStringSetting(ctx, s.UserId(ctx), field, val)
			if err != nil {
				whfatal.Error(err)
			}
		}
	}

	whfatal.Redirect("/settings")
}

func (s *Site) Settings(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	srv, err := calendar.New(s.OAuth2Client(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	type calendarData struct {
		*calendar.CalendarListEntry
		Enabled bool
	}

	var calendars []*calendarData
	err = srv.CalendarList.List().MinAccessRole("writer").Pages(ctx,
		func(l *calendar.CalendarList) error {
			for _, item := range l.Items {
				channels, err := s.db.GetChannels(ctx, s.UserId(ctx), item.Id)
				if err != nil {
					return err
				}

				calendars = append(calendars, &calendarData{
					CalendarListEntry: item,
					Enabled:           len(channels) > 0,
				})
			}
			return nil
		})
	if err != nil {
		whfatal.Error(err)
	}

	sort.Slice(calendars, func(i, j int) bool {
		if calendars[i].Enabled && !calendars[j].Enabled {
			return true
		}
		if !calendars[i].Enabled && calendars[j].Enabled {
			return false
		}
		if calendars[i].Primary && !calendars[j].Primary {
			return true
		}
		if !calendars[i].Primary && calendars[j].Primary {
			return false
		}
		if calendars[i].AccessRole == "owner" && calendars[j].AccessRole != "owner" {
			return true
		}
		if calendars[i].AccessRole != "owner" && calendars[j].AccessRole == "owner" {
			return false
		}
		return calendars[i].Id < calendars[j].Id
	})

	values := map[string]interface{}{
		"Calendars": calendars,
	}

	for _, field := range []string{"autoreject_name", "autoreject_reply"} {
		val, err := s.db.GetStringSetting(ctx, s.UserId(ctx), field)
		if err != nil {
			whfatal.Error(err)
		}
		values[field] = val
	}

	s.r.Render(w, r, "settings", values)
}

func (s *Site) LoginRequired(h http.Handler) http.Handler {
	return s.r.Provider.LoginRequired(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ctx := whcompat.Context(r)

			tok := s.OAuth2Token(ctx)
			if tok.RefreshToken != "" {
				err := s.db.SetUserOAuth2Token(ctx, s.UserId(ctx), tok)
				if err != nil {
					whfatal.Error(err)
				}
			}

			h.ServeHTTP(w, r)
		}))
}

func main() {
	ctx := context.Background()
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
	db, err := NewDB(ctx, gcpProjectId)
	if err != nil {
		panic(err)
	}
	site := &Site{r: rend, db: db}

	panic(whlog.ListenAndServe(listenAddr(),
		whcache.Register(
			whsess.HandlerWithStore(whsess.NewCookieStore(cookieSecret),
				whfatal.Catch(
					whmux.Dir{
						"":      whmux.Exact(rend.Simple("index")),
						"event": http.HandlerFunc(site.Event),
						"cron":  http.HandlerFunc(site.Cron),
						"settings": site.LoginRequired(whmux.ExactPath(
							whmux.Method{
								"GET":  http.HandlerFunc(site.Settings),
								"POST": http.HandlerFunc(site.UpdateSettings),
							})),
						"register": site.LoginRequired(whmux.ExactPath(
							whmux.RequireMethod("POST",
								http.HandlerFunc(site.Register)))),
						"unregister": site.LoginRequired(whmux.ExactPath(
							whmux.RequireMethod("POST",
								http.HandlerFunc(site.Unregister)))),
						"auth": oauth})))))
}
