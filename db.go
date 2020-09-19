package main

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
)

// User keys should be NameKey("User", userId, nil)

type DSChannel struct {
	// Datastore Key should be NameKey("Channel", channelId, userKey)
	CalId      string
	ResourceId string
}

type DSOAuth2Token struct {
	// Datastore Key should be NameKey("OAuth2Token", "oauth2_token",  userKey)
	OAuth2Token []byte `datastore:",noindex"`
}

type DSConflictName struct {
	// Datastore Key should be either
	// IDKey("ConflictName", id, userKey)
	// IncompleteKey("ConflictName", userKey)
	Name string
}

type DB struct {
	datastore *datastore.Client
}

func NewDB(ctx context.Context, gcpProjectId string) (*DB, error) {
	cli, err := datastore.NewClient(ctx, gcpProjectId)
	if err != nil {
		return nil, err
	}
	return &DB{datastore: cli}, nil
}

func (d *DB) userKey(userId string) *datastore.Key {
	return datastore.NameKey("User", userId, nil)
}

func (d *DB) oauth2TokenKey(userId string) *datastore.Key {
	return datastore.NameKey("OAuth2Token", "oauth2_token", d.userKey(userId))
}

func (d *DB) channelKey(userId string, channelId string) *datastore.Key {
	return datastore.NameKey("Channel", channelId, d.userKey(userId))
}

func (d *DB) SetUserOAuth2Token(ctx context.Context, userId string,
	tok *oauth2.Token) error {
	data, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	_, err = d.datastore.Put(ctx, d.oauth2TokenKey(userId), &DSOAuth2Token{
		OAuth2Token: data,
	})
	return err
}

func (d *DB) GetUserOAuth2Token(ctx context.Context, userId string) (
	*oauth2.Token, error) {
	var val DSOAuth2Token
	err := d.datastore.Get(ctx, d.oauth2TokenKey(userId), &val)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	err = json.Unmarshal(val.OAuth2Token, &tok)
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

type StoppableChannel struct {
	ChannelId  string
	ResourceId string
}

func (d *DB) GetChannels(ctx context.Context, userId string, calId string) (
	[]StoppableChannel, error) {
	it := d.datastore.Run(ctx, datastore.NewQuery("Channel").
		Ancestor(d.userKey(userId)).Filter("CalId =", calId))
	var chans []StoppableChannel
	for {
		var ch DSChannel
		key, err := it.Next(&ch)
		if err == iterator.Done {
			return chans, nil
		}
		if err != nil {
			return chans, err
		}
		chans = append(chans, StoppableChannel{
			ChannelId:  key.Name,
			ResourceId: ch.ResourceId,
		})
	}
}

func (d *DB) AddChannel(ctx context.Context, userId, chanId, calId, resourceId string) error {
	_, err := d.datastore.Put(ctx, d.channelKey(userId, chanId), &DSChannel{
		CalId:      calId,
		ResourceId: resourceId,
	})
	return err
}

func (d *DB) RemoveChannel(ctx context.Context, userId, chanId string) error {
	err := d.datastore.Delete(ctx, d.channelKey(userId, chanId))
	return err
}
