package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
)

// User keys should be NameKey("User", userId, nil)

type DSChannel struct {
	// Datastore Key should be NameKey("Channel", channelId, nil)
	UserId     string
	CalId      string
	ResourceId string
	Expiration time.Time

	// TODO: cron job to unexpire
}

var DefaultConfigValues = map[string]string{
	"autoreject_name":  "(autoreject)",
	"autoreject_reply": "Automatic decline - unavailable. Please ask about scheduling during this block of time.",
}

type DSConfigString struct {
	// Datastore Key should be NameKey("ConfigString", name, userKey)
	Value string `datastore:",noindex"`
	// Settings:
	// * autoreject_name
	// * autoreject_reply
	// * syncstart-<calid>
	// * synctoken-<calid>
}

type DSConfigBytes struct {
	// Datastore Key should be NameKey("ConfigBytes", name, userKey)
	Value []byte `datastore:",noindex"`
	// Settings:
	// * oauth2_token
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

func (d *DB) configBytesKey(userId, name string) *datastore.Key {
	return datastore.NameKey("ConfigBytes", name, d.userKey(userId))
}

func (d *DB) configStringKey(userId, name string) *datastore.Key {
	return datastore.NameKey("ConfigString", name, d.userKey(userId))
}

func (d *DB) channelKey(channelId string) *datastore.Key {
	return datastore.NameKey("Channel", channelId, nil)
}

func (d *DB) SetUserOAuth2Token(ctx context.Context, userId string,
	tok *oauth2.Token) error {
	data, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	_, err = d.datastore.Put(ctx,
		d.configBytesKey(userId, "oauth2_token"),
		&DSConfigBytes{Value: data})
	return err
}

func (d *DB) GetUserOAuth2Token(ctx context.Context, userId string) (
	*oauth2.Token, error) {
	var val DSConfigBytes
	err := d.datastore.Get(ctx, d.configBytesKey(userId, "oauth2_token"), &val)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	err = json.Unmarshal(val.Value, &tok)
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
		Filter("UserId =", userId).Filter("CalId =", calId))
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

func (d *DB) GetChannel(ctx context.Context, chanId string) (*DSChannel, error) {
	var val DSChannel
	return &val, d.datastore.Get(ctx, d.channelKey(chanId), &val)
}

func (d *DB) AddChannel(ctx context.Context, userId, chanId, calId, resourceId string, expiration time.Time) error {
	_, err := d.datastore.Put(ctx, d.channelKey(chanId), &DSChannel{
		UserId:     userId,
		CalId:      calId,
		ResourceId: resourceId,
		Expiration: expiration,
	})
	return err
}

func (d *DB) RemoveChannel(ctx context.Context, chanId string) error {
	err := d.datastore.Delete(ctx, d.channelKey(chanId))
	return err
}

func (d *DB) GetStringSetting(ctx context.Context, userId, name string) (string, error) {
	var val DSConfigString
	err := d.datastore.Get(ctx, d.configStringKey(userId, name), &val)
	if err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return DefaultConfigValues[name], nil
		}
		return "", err
	}
	return val.Value, nil
}

func (d *DB) SetStringSetting(ctx context.Context, userId, name, value string) error {
	_, err := d.datastore.Put(ctx, d.configStringKey(userId, name), &DSConfigString{Value: value})
	return err
}
