package main

import (
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
	"gopkg.in/webhelp.v1/whcompat"
	"gopkg.in/webhelp.v1/whfatal"
)

func (s *Site) Event(w http.ResponseWriter, r *http.Request) {
	ctx := whcompat.Context(r)
	chanId := r.Header.Get("X-Goog-Channel-ID")
	channel, err := s.db.GetChannel(ctx, chanId)
	if err != nil {
		whfatal.Error(err)
	}

	syncToken, err := s.db.GetStringSetting(ctx, channel.UserId,
		"synctoken-"+channel.CalId)
	if err != nil {
		whfatal.Error(err)
	}

	autorejectName, err := s.db.GetStringSetting(ctx, channel.UserId,
		"autoreject_name")
	if err != nil {
		whfatal.Error(err)
	}
	autorejectName = strings.ToLower(strings.TrimSpace(autorejectName))

	autorejectReply, err := s.db.GetStringSetting(ctx, channel.UserId,
		"autoreject_reply")
	if err != nil {
		whfatal.Error(err)
	}

	oldestCreationStr, err := s.db.GetStringSetting(ctx, channel.UserId,
		"syncstart-"+channel.CalId)
	if err != nil {
		whfatal.Error(err)
	}
	oldestCreation, err := time.Parse(time.RFC3339, oldestCreationStr)
	if err != nil {
		whfatal.Error(err)
	}

	tok, err := s.db.GetUserOAuth2Token(ctx, channel.UserId)
	if err != nil {
		whfatal.Error(err)
	}

	srv, err := calendar.New(s.r.Provider.Provider().Config.Client(ctx, tok))
	if err != nil {
		whfatal.Error(err)
	}

	nextSyncToken, err := RejectBadInvites(
		ctx, srv, channel.CalId, syncToken, func(e *calendar.Event) bool {
			if !strings.Contains(strings.ToLower(e.Summary), autorejectName) {
				return false
			}
			if len(e.Attendees) != 0 {
				return false
			}
			return true
		}, autorejectReply,
		oldestCreation)
	if err != nil {
		whfatal.Error(err)
	}

	err = s.db.SetStringSetting(ctx, channel.UserId,
		"synctoken-"+channel.CalId, nextSyncToken)
	if err != nil {
		whfatal.Error(err)
	}
}
