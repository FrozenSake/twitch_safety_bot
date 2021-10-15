package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/nicklaw5/helix"
)

func subscribeToFollows(broadcasterID string) {
	resp, err := HELIXCLIENT.CreateEventSubSubscription(&helix.EventSubSubscription{
		Type:    helix.EventSubTypeChannelFollow,
		Version: "1",
		Condition: helix.EventSubCondition{
			BroadcasterUserID: fmt.Sprintf("%s", broadcasterID),
		},
		Transport: helix.EventSubTransport{
			Method:   "webhook",
			Callback: fmt.Sprintf("https://%s/follow", "TODO"), //TODO
			Secret:   fmt.Sprintf("%s", "TODO"),                //TODO
		},
	})
	if err != nil {
		// handle error
	}

	fmt.Printf("%+v\n", resp)
}

type eventSubNotification struct {
	Subscription helix.EventSubSubscription `json:"subscription"`
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
}

func eventsubFollow(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}
	defer r.Body.Close()
	// verify that the notification came from twitch using the secret.
	if !helix.VerifyEventSubNotification("s3cre7w0rd", r.Header, string(body)) {
		log.Println("no valid signature on subscription")
		return
	} else {
		log.Println("verified signature for subscription")
	}
	var vals eventSubNotification
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&vals)
	if err != nil {
		log.Println(err)
		return
	}
	// if there's a challenge in the request, respond with only the challenge to verify your eventsub.
	if vals.Challenge != "" {
		w.Write([]byte(vals.Challenge))
		return
	}
	var followEvent helix.EventSubChannelFollowEvent
	err = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&followEvent)

	log.Printf("got follow webhook: %s follows %s\n", followEvent.UserName, followEvent.BroadcasterUserName)
	if checkUserBanStatus(followEvent.UserName) {
		banIRCUser(followEvent.UserName, followEvent.BroadcasterUserName)
	} else {
		safelistUser(followEvent.UserName)
	}
	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

func unsubscribeFromFollows() {
	client, err := helix.NewClient(&helix.Options{
		ClientID:       "your-client-id",
		AppAccessToken: "your-app-access-token",
	})
	if err != nil {
		// handle error
	}

	resp, err := client.RemoveEventSubSubscription("subscription-id")
	if err != nil {
		// handle error
	}

	fmt.Printf("%+v\n", resp)
}
