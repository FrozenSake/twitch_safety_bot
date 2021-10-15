package main

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

func joinChannels(targets []string) map[string]broadcaster {
	zap.S().Info("Prepare channels")
	channelsStatus := make(map[string]broadcaster)
	for _, channelName := range targets {
		channelName = strings.ToLower(channelName)
		bc := broadcaster{name: channelName, connected: false}
		connectChannel(bc)

		channelsStatus[channelName] = bc
	}
	return channelsStatus
}

// disconnectTwitchChannel takes a broadcaster and disconnects
// from their IRC channel.
func disconnectTwitchChannel(ch broadcaster) {
	zap.S().Infof("Departing channel %v", ch.name)
	IRCCLIENT.Depart(ch.name)
	ch.connected = false
}

// connectTwitchChannel takes a broadcaster and connects
// to their IRC channel.
func connectChannel(ch broadcaster) {
	zap.S().Infof("Joining channel %v", ch.name)
	IRCCLIENT.Join(ch.name)
	ch.connected = true
}

func getUserList(channelName string) []string {
	zap.S().Debugf("## USERLIST FOR %v ##\n", channelName)
	userlist, err := IRCCLIENT.Userlist(channelName)
	if err != nil {
		zap.S().Errorf("Encountered error listing users: %v", err)
	}
	zap.S().Debugf("Users: %v\n", userlist)

	return userlist
	// for _, val := range userlist {
	// 	if strings.Contains(userSafelist, val) {
	// 		continue
	// 	}
	// 	if checkUserBanStatus(val) {
	// 		banUser(val, channelName)
	// 	} else {
	// 		safelistUser(val)
	// 	}
	// }
}

func banIRCUser(user string, channelName string) {
	banMessage := fmt.Sprintf("/ban %s", user)
	IRCCLIENT.Say(channelName, banMessage)

	addUserToBanList(user)

	bannedMessage := fmt.Sprintf("banned %s", user)
	IRCCLIENT.Say(channelName, bannedMessage)

	blockUser(user)
}
