package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

func parseTime(eventTime *calendar.EventDateTime, isStart bool) (time.Time, error) {
	loc := time.UTC
	if eventTime.TimeZone != "" {
		var err error
		loc, err = time.LoadLocation(eventTime.TimeZone)
		if err != nil {
			return time.Time{}, Err.Wrap(err)
		}
	}
	if eventTime.DateTime != "" {
		rv, err := time.ParseInLocation(time.RFC3339, eventTime.DateTime, loc)
		return rv, Err.Wrap(err)
	}
	if eventTime.Date == "" {
		return time.Time{}, Err.Wrap(fmt.Errorf("no datetime or date"))
	}
	if isStart {
		rv, err := time.ParseInLocation("2006-01-02T15:04:05", eventTime.Date+"T00:00:00", loc)
		return rv, Err.Wrap(err)
	}
	rv, err := time.ParseInLocation("2006-01-02T15:04:05", eventTime.Date+"T23:59:59", loc)
	return rv, Err.Wrap(err)
}

func RejectBadInvites(ctx context.Context, srv *calendar.Service,
	calId, lastSyncToken string, autorejectMatcher func(e *calendar.Event) bool,
	autorejectComment string, oldestCreation time.Time) (
	nextSyncToken string, err error) {
	callback := func(e *calendar.Events) error {
		nextSyncToken = e.NextSyncToken
		for _, item := range e.Items {
			if len(item.Attendees) != 1 {
				continue
			}
			if item.Attendees[0].ResponseStatus != "needsAction" {
				continue
			}
			createdTime, err := time.Parse(time.RFC3339, item.Created)
			if err != nil {
				return Err.Wrap(err)
			}
			if createdTime.Before(oldestCreation) {
				continue
			}
			if item.Start.DateTime == "" || item.End.DateTime == "" {
				continue
			}

			itemStart, err := parseTime(item.Start, true)
			if err != nil {
				return err
			}
			itemEnd, err := parseTime(item.End, false)
			if err != nil {
				return err
			}

			conflictFound := false
			err = srv.Events.List(calId).
				SingleEvents(true).
				MaxAttendees(1).
				TimeMin(itemStart.Add(-time.Hour*25).Format(time.RFC3339)).
				TimeMax(itemEnd.Add(time.Hour*25).Format(time.RFC3339)).
				OrderBy("startTime").
				Pages(ctx,
					func(e *calendar.Events) error {
						for _, conflict := range e.Items {
							if !autorejectMatcher(conflict) {
								continue
							}
							conflictStart, err := parseTime(conflict.Start, true)
							if err != nil {
								return err
							}
							conflictEnd, err := parseTime(conflict.End, false)
							if err != nil {
								return err
							}
							if conflictStart.After(itemEnd) || conflictStart.Equal(itemEnd) {
								continue
							}
							if conflictEnd.Before(itemStart) || conflictEnd.Equal(itemStart) {
								continue
							}
							conflictFound = true
						}
						return nil
					})
			if err != nil {
				return Err.Wrap(err)
			}
			if conflictFound {
				_, err = srv.Events.Patch(calId, item.Id, &calendar.Event{
					Id:    item.Id,
					Start: item.Start,
					End:   item.End,
					Attendees: []*calendar.EventAttendee{
						{
							Email:          item.Attendees[0].Email,
							Comment:        autorejectComment,
							Id:             item.Attendees[0].Id,
							ResponseStatus: "declined",
						}},
				}).Context(ctx).SendUpdates("all").Do()
				if err != nil {
					return Err.Wrap(err)
				}
			}
		}
		return nil
	}

	err = srv.Events.List(calId).SyncToken(lastSyncToken).
		MaxAttendees(1).SingleEvents(true).Pages(ctx, callback)
	if err != nil {
		if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == http.StatusGone {
			return RejectBadInvites(
				ctx, srv, calId, "", autorejectMatcher, autorejectComment, oldestCreation)
		}
		return "", Err.Wrap(err)
	}
	return nextSyncToken, nil
}
