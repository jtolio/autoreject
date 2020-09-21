package main

import (
	"net/http"
	"time"

	"google.golang.org/api/calendar/v3"
	"gopkg.in/webhelp.v1/whcompat"
	"gopkg.in/webhelp.v1/whfatal"
)

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

	err = s.db.AddChannel(ctx, s.UserId(ctx), chanId, calId, channel.ResourceId,
		time.Unix(0, channel.Expiration*int64(time.Millisecond)))
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
		err = s.db.RemoveChannel(ctx, channel.ChannelId)
		if err != nil {
			whfatal.Error(err)
		}
	}

	whfatal.Redirect("/settings")
}
