package main

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/calendar/v3"
	"gopkg.in/webhelp.v1/whcompat"
	"gopkg.in/webhelp.v1/whfatal"
)

func (s *Site) Cron(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)

	expiringSoon := time.Now().Add(48 * time.Hour)
	err := s.db.AllChannels(ctx,
		func(ctx context.Context, key *datastore.Key, ch *DSChannel) error {
			if ch.Expiration.After(expiringSoon) {
				return s.sync(ctx, key.Name, ch)
			}

			tok, err := s.db.GetUserOAuth2Token(ctx, ch.UserId)
			if err != nil {
				return err
			}
			srv, err := calendar.New(s.r.Provider.Provider().Config.Client(ctx, tok))
			if err != nil {
				return err
			}

			err = s.addChannel(ctx, srv, ch.CalId, ch.UserId)
			if err != nil {
				return err
			}

			err = s.removeChannel(ctx, srv, key.Name, ch.ResourceId)
			if err != nil {
				return err
			}

			return nil
		})
	if err != nil {
		whfatal.Error(err)
	}

	w.Write([]byte("success"))
}
