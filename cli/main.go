package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kr/pretty"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

func parseTime(eventTime *calendar.EventDateTime, isStart bool) (time.Time, error) {
	loc := time.UTC
	if eventTime.TimeZone != "" {
		var err error
		loc, err = time.LoadLocation(eventTime.TimeZone)
		if err != nil {
			return time.Time{}, err
		}
	}
	if eventTime.DateTime != "" {
		return time.ParseInLocation(time.RFC3339, eventTime.DateTime, loc)
	}
	if eventTime.Date == "" {
		return time.Time{}, fmt.Errorf("no datetime or date")
	}
	if isStart {
		return time.ParseInLocation("2006-01-02T15:04:05", eventTime.Date+"T00:00:00", loc)
	}
	return time.ParseInLocation("2006-01-02T15:04:05", eventTime.Date+"T23:59:59", loc)
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	ctx := context.Background()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	autorejectSummary := "autoreject"

	lastSyncToken := "CNCanMzE-esCENCanMzE-esCGAU="
	for {
		err = srv.Events.List("primary").
			SyncToken(lastSyncToken).
			MaxAttendees(1).
			SingleEvents(true).
			Pages(ctx,
				func(e *calendar.Events) error {
					lastSyncToken = e.NextSyncToken
					for _, item := range e.Items {
						if len(item.Attendees) != 1 {
							continue
						}
						if item.Attendees[0].ResponseStatus != "needsAction" {
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
						err = srv.Events.List("primary").
							SingleEvents(true).
							MaxAttendees(1).
							TimeMin(itemStart.Add(-time.Hour*25).Format(time.RFC3339)).
							TimeMax(itemEnd.Add(time.Hour*25).Format(time.RFC3339)).
							OrderBy("startTime").
							Pages(ctx,
								func(e *calendar.Events) error {
									for _, conflict := range e.Items {
										if conflict.Summary != autorejectSummary {
											continue
										}
										if len(conflict.Attendees) != 0 {
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
							return err
						}
						if conflictFound {
							fmt.Println("should schedule rejection")
						} else {
							fmt.Println("should ignore")
						}
					}
					return nil
				})
		if err != nil {
			if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == http.StatusGone {
				lastSyncToken = ""
				continue
			}
			panic(pretty.Sprint(err))
		}
		fmt.Println("waiting for more", lastSyncToken)
		time.Sleep(10 * time.Second)
	}
}
