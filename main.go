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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

func getClient(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	tok, err := tokenFromFile("token.json")
	if err != nil {
		tok, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, err
		}
		err = saveToken("token.json", tok)
		if err != nil {
			log.Printf("failed saving token: %v", err)
		}
	}
	return config.Client(ctx, tok), nil
}

func tokenFromFile(path string) (*oauth2.Token, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	var tok oauth2.Token
	err = json.NewDecoder(fh).Decode(&tok)
	return &tok, err
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to %v and then type the code:", authURL)
	var authCode string
	_, err := fmt.Scan(&authCode)
	if err != nil {
		return nil, err
	}
	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, err
	}
	return tok, nil
}

func saveToken(path string, tok *oauth2.Token) error {
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(fh).Encode(tok)
	if err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

func main() {
	err := Main(context.Background())
	if err != nil {
		panic(err)
	}
}

func Main(ctx context.Context) error {
	creds, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		return err
	}
	config, err := google.ConfigFromJSON(creds, calendar.CalendarReadonlyScope)
	if err != nil {
		return err
	}
	client, err := getClient(ctx, config)
	if err != nil {
		return err
	}
	srv, err := calendar.New(client)
	if err != nil {
		return err
	}
	events, err := srv.Events.List("primary").
		ShowDeleted(false).SingleEvents(true).
		TimeMin(time.Now().Format(time.RFC3339)).
		MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		return err
	}
	for _, item := range events.Items {
		date := item.Start.DateTime
		if date == "" {
			date = item.Start.Date
		}
		fmt.Printf("%v (%v)\n", item.Summary, date)
	}
	return nil
}
