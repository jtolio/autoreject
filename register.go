package main

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/api/calendar/v3"
	"gopkg.in/webhelp.v1/whcompat"
	"gopkg.in/webhelp.v1/whfatal"
)

func (s *Site) addChannel(ctx context.Context, srv *calendar.Service,
	calId, userId string) error {
	chanId := idGen()
	channel, err := srv.Events.Watch(calId, &calendar.Channel{
		Address: baseURL + "/event",
		Id:      chanId,
		Type:    "web_hook",
	}).Context(ctx).Do()
	if err != nil {
		return err
	}
	return s.db.AddChannel(ctx, userId, chanId, calId, channel.ResourceId,
		time.Unix(0, channel.Expiration*int64(time.Millisecond)))
}

func (s *Site) Register(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	srv, err := calendar.New(s.OAuth2Client(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	calId := r.FormValue("cal")

	err = s.addChannel(ctx, srv, calId, s.UserId(ctx))
	if err != nil {
		whfatal.Error(err)
	}

	err = s.db.SetStringSetting(ctx, s.UserId(ctx),
		"syncstart-"+calId, time.Now().Format(time.RFC3339))
	if err != nil {
		whfatal.Error(err)
	}

	whfatal.Redirect("/settings")
}

func (s *Site) removeChannel(ctx context.Context, srv *calendar.Service,
	chanId, resourceId string) error {
	err := srv.Channels.Stop(&calendar.Channel{
		Id:         chanId,
		ResourceId: resourceId,
	}).Context(ctx).Do()
	if err != nil {
		return err
	}
	return s.db.RemoveChannel(ctx, chanId)
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
		err = s.removeChannel(ctx, srv, channel.ChannelId, channel.ResourceId)
		if err != nil {
			whfatal.Error(err)
		}
	}

	whfatal.Redirect("/settings")
}
