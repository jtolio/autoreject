package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"

	"github.com/jtolio/autoreject/views"
	"github.com/kr/pretty"
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

	var calendars []calendarData
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

				channels, err := s.db.GetChannels(ctx, s.UserId(ctx), item.Id)
				if err != nil {
					return err
				}

				calendars = append(calendars, calendarData{
					CalendarListEntry: item,
					Enabled:           len(channels) > 0,
				})
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

func (s *Site) Event(w http.ResponseWriter, r *http.Request) {
	whlog.Default("event: %s", pretty.Sprint(r))
}

func (s *Site) Register(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	srv, err := calendar.New(s.OAuth2Client(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	chanId := idGen()
	calId := r.FormValue("cal")

	channel, err := srv.Events.Watch(calId, &calendar.Channel{
		Address: baseURL + "/event",
		Id:      chanId,
		Type:    "web_hook",
	}).Context(ctx).Do()
	if err != nil {
		whfatal.Error(err)
	}

	err = s.db.AddChannel(ctx, s.UserId(ctx), chanId, calId, channel.ResourceId)
	if err != nil {
		whfatal.Error(err)
	}

	whfatal.Redirect("/settings")
}

func (s *Site) Unregister(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	channels, err := s.db.GetChannels(ctx, s.UserId(ctx), r.FormValue("cal"))
	if err != nil {
		whfatal.Error(err)
	}

	srv, err := calendar.New(s.OAuth2Client(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	for _, channel := range channels {
		err = srv.Channels.Stop(&calendar.Channel{
			Id:         channel.ChannelId,
			ResourceId: channel.ResourceId,
		}).Context(ctx).Do()
		if err != nil {
			whfatal.Error(err)
		}
		err = s.db.RemoveChannel(ctx, s.UserId(ctx), channel.ChannelId)
		if err != nil {
			whfatal.Error(err)
		}
	}

	whfatal.Redirect("/settings")
}

func (s *Site) LoginRequired(h http.Handler) http.Handler {
	return s.r.Provider.LoginRequired(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ctx := whcompat.Context(r)

			err := s.db.SetUserOAuth2Token(ctx, s.UserId(ctx), s.OAuth2Token(ctx))
			if err != nil {
				whfatal.Error(err)
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
		whlog.LogRequests(whlog.Default, whlog.LogResponses(whlog.Default,
			whcache.Register(
				whsess.HandlerWithStore(whsess.NewCookieStore(cookieSecret),
					whfatal.Catch(
						whmux.Dir{
							"":      whmux.Exact(rend.Simple("index")),
							"event": http.HandlerFunc(site.Event),
							"settings": site.LoginRequired(whmux.Exact(
								http.HandlerFunc(site.Settings))),
							"register": site.LoginRequired(whmux.ExactPath(
								whmux.RequireMethod("POST",
									http.HandlerFunc(site.Register)))),
							"unregister": site.LoginRequired(whmux.ExactPath(
								whmux.RequireMethod("POST",
									http.HandlerFunc(site.Unregister)))),
							"auth": oauth})))))))
}
