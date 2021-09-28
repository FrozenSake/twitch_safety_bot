package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v2"
	"github.com/nicklaw5/helix"
	"github.com/spf13/viper"

	"go.uber.org/zap"
)

const (
	oauthForm     = "oauth:"
	banListsLoc   = "./lists"
	banFiltersLoc = "./filters"
	banLogsLoc    = "./logs"
)

type broadcaster struct {
	name      string
	connected bool
}

var IRCCLIENT *twitch.Client
var HELIXCLIENT *helix.Client

var (
	banLog        string
	banList       string
	banFilters    string
	newFilters    string
	userWhitelist string
)

/*

Use userid and block user
	need to queue it up so we don't hit rate limits
	channel?


Setup local HTTP server as part of bot, for oauth and callbacks
localhost/auth  --> handler for oauth2 flow
ip:443/follow   --> handler for follow event webhook

*/

/* General Twitch */

func OauthCheck(oauth string) {
	zap.S().Debug("Checking OAuth Format")
	if oauth[:6] != oauthForm {
		zap.S().Debug("Fixing OAuth Format")
		oauth = oauthForm + oauth
	}
}

func Disconnectedchannel(ch broadcaster) {
	ch.connected = false
}

func ConnectedChannel(ch broadcaster) {
	ch.connected = true
}

func getAndCheckUserList(channelName string) {
	zap.S().Debugf("##USERLIST FOR %v##\n", channelName)
	userlist, err := IRCCLIENT.Userlist(channelName)
	if err != nil {
		zap.S().Errorf("Encountered error listing users: %v", err)
		zap.S().Infof("Skipping to next channel")
	}
	zap.S().Debugf("Users: %v\n", userlist)
	for _, val := range userlist {
		if strings.Contains(userWhitelist, val) {
			continue
		}
		if checkIfNameFiltered(val) {
			banUser(val, channelName)
		} else {
			whitelistUser(val)
		}
	}
}

func banUser(user string, channelName string) {
	banMessage := fmt.Sprintf("/ban %s", user)
	IRCCLIENT.Say(channelName, banMessage)
	if banLog != "" {
		banLog = fmt.Sprintf("%s\n%s", banLog, user)
	} else {
		banLog = fmt.Sprintf("%s", user)
	}

	// Get ID and Block User Codeblock
	var names []string
	names = append(names, user)
	ids, err := getUserIDs(names)
	if err != nil {
		// Handle it
	}
	for _, id := range ids {
		err := blockUser(id)
		if err != nil {
			// Handle it
		}
	}
}

func whitelistUser(user string) {
	zap.S().Infof("Whitelisting user %s for this stream.", user)
	if userWhitelist != "" {
		userWhitelist = fmt.Sprintf("%s\n%s", userWhitelist, user)
	} else {
		userWhitelist = fmt.Sprintf("%s", user)
	}
}

func getUserIDs(loginNames []string) ([]string, error) {
	resp, err := HELIXCLIENT.GetUsers(&helix.UsersParams{
		Logins: loginNames,
	})
	if err != nil {
		return nil, err
	}
	fmt.Printf("%v", resp)
	var ids []string
	for _, v := range resp.Data.Users {
		ids = append(ids, v.ID)
	}
	return ids, nil
}

func blockUser(userID string) error {
	resp, err := HELIXCLIENT.BlockUser(&helix.BlockUserParams{
		TargetUserID:  fmt.Sprintf("%s", userID),
		SourceContext: "chat",
		Reason:        "Banned by Safety Bot for matching safety filters.",
	})
	if err != nil || resp.StatusCode != 204 {
		zap.S().Errorf("Error blocking userID: %s", userID)
		return err
	}
	zap.S().Infof("Blocked userID: %s", userID)
	return nil
}

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
	if checkIfNameFiltered(followEvent.UserName) {
		banUser(followEvent.UserName, followEvent.BroadcasterUserName)
	} else {
		whitelistUser(followEvent.UserName)
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

/* Name Parsing */

func checkIfNameFiltered(name string) bool {
	if banList == "" {
		zap.S().Info("No banned usernames, if this is expected, please ignore.")
	}
	if strings.Contains(banList, name) {
		zap.S().Infof("Found the name %s in the ban lists.", name)
		return true
	}
	if banFilters == "" {
		zap.S().Info("No filters found, if this is expected, please ignore.")
	}
	filters := strings.Split(banFilters, "\n")
	for _, filter := range filters {
		if filter == "" {
			continue
		}
		if filter[:1] != "^" {
			//zap.S().Infof("Adding ^ to filter: %s", filter)
			filter = fmt.Sprintf("^%s", filter)
		}
		re := regexp.MustCompile(filter)
		if re.Match([]byte(name)) {
			zap.S().Infof("The name %s matched one of the ban filters.", name)
			return true
		}
	}
	return false
}

/* Environment Variables & Config File */

func buildConfig() {
	fmt.Println("Preparing a config file. NOTE: CREDENTIALS WILL BE SAVED LOCALLY.")
	fmt.Println("Enter the twitch username for the bot to connect to: ")
	var username string
	fmt.Scanln(&username)
	viper.Set("username", username)
	fmt.Println("Enter the oauth token for the bot to authenticate with: ")
	var oauth string
	fmt.Scanln(&oauth)
	viper.Set("oauth", oauth)
	fmt.Println("Enter a comma separated list of channels for the bot to connect to: ")
	var channels string
	fmt.Scanln(&channels)
	viper.Set("channels", channels)
	viper.WriteConfig()
	fmt.Println("Config file written to .env")
}

/* File IO */

func prepareLists() (string, string) {
	banList, err := readLists(banListsLoc)
	if err != nil {
		zap.S().Errorf("Error in reading list at %s", banListsLoc)
	}
	banFilters, err := readLists(banFiltersLoc)
	if err != nil {
		zap.S().Errorf("Error in reading list at %s", banFiltersLoc)
	}
	return banList, banFilters
}

func readLists(folder string) (string, error) {
	list := ""
	pathtofiles := fmt.Sprintf("./%s", folder)
	err := filepath.Walk(pathtofiles, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if !info.IsDir() {
			list = list + readFileAsString(path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", pathtofiles, err)
		return "", err
	}
	return list, nil
}

func readFileAsString(path string) string {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		zap.S().Errorf("Reading file failed with error: %s", err)
	}
	return string(buf)
}

func writeBans() {
	if _, err := os.Stat(banLogsLoc); os.IsNotExist(err) {
		os.Mkdir(banLogsLoc, 0777)
	}
	t := time.Now()
	if banLog != "" {
		banLogBytes := []byte(banLog)
		path := fmt.Sprintf("%s/banlog-%s", banLogsLoc, t.Format("02Jan06-15:04MST"))
		err := os.WriteFile(path, banLogBytes, 0666)
		if err != nil {
			zap.S().Errorf("Error writing banlog to %s", path)
		}
	}
	if newFilters != "" {
		newFiltersBytes := []byte(newFilters)
		filterpath := fmt.Sprintf("%s/newfilters-%s", banLogsLoc, t.Format("02Jan06-15:04MST"))
		err := os.WriteFile(filterpath, newFiltersBytes, 0666)
		if err != nil {
			zap.S().Errorf("Error writing filter log to %s", filterpath)
		}
	}
}

/* Run */

func main() {
	sugar, _ := zap.NewDevelopment()
	defer sugar.Sync()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		zap.S().Info("Writing out state and closing.")
		writeBans()
		os.Exit(1)
	}()
	ticker := time.NewTicker(60 * time.Second)

	userlistChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				getAndCheckUserList("hikthur")
			case <-userlistChan:
				ticker.Stop()
				return
			}
		}
	}()
	defer close(userlistChan)

	zap.ReplaceGlobals(sugar)

	zap.S().Info("Twitch Saftey Bot Starting up.")

	zap.S().Debug("Setting Environment Variables from .env")
	viper.SetConfigFile(".env")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			zap.S().Info("Config file not found, prompting user for values.")
			buildConfig()
			if err = viper.ReadInConfig(); err != nil {
				zap.S().Errorf("Could not read or generate a config file: %s", err)
				panic(err)
			}
		} else {
			zap.S().Errorf("Something went wrong with viper: %s", ok)
		}
	}

	username := viper.GetString("username")
	oauth := viper.GetString("oauth")
	channels := viper.GetString("channels")
	targets := strings.Split(channels, ",")
	OauthCheck(oauth)
	channelsStatus := make(map[string]broadcaster)

	// Define a regex object
	filterre := regexp.MustCompile("^!FilterAdd (?P<filter>\\S*)")
	banre := regexp.MustCompile("^!BanAdd (?P<ban>\\S*)")

	banList, banFilters = prepareLists()
	userWhitelist = ""
	banLog = ""
	newFilters = ""

	zap.S().Infof("Creating Twitch IRCCLIENT: %v", username)
	IRCCLIENT = twitch.NewClient(username, oauth)

	zap.S().Info("Prepare channels")
	for _, channelName := range targets {
		channelName = strings.ToLower(channelName)
		zap.S().Infof("Joining channel %v", channelName)
		IRCCLIENT.Join(channelName)

		bc := broadcaster{name: channelName, connected: true}
		channelsStatus[channelName] = bc
	}

	//TODO -> move banning to go channel
	IRCCLIENT.OnPrivateMessage(func(message twitch.PrivateMessage) {
		zap.S().Infof("User %s joined the channel", message.User.Name)
		if len(message.User.Badges) == 0 && !strings.Contains(userWhitelist, message.User.Name) {
			zap.S().Info("No badges")
			if checkIfNameFiltered(message.User.Name) {
				banUser(message.User.Name, message.Channel)
			} else {
				whitelistUser(message.User.Name)
			}
		}
		allowed := false
		if _, ok := message.User.Badges["moderator"]; ok {
			allowed = true
		}
		if _, ok := message.User.Badges["broadcaster"]; ok {
			allowed = true
		}
		if _, ok := message.User.Badges["vip"]; ok {
			allowed = true
		}
		if !allowed {
			return
		}
		if banre.MatchString(message.Message) {
			target := message.Channel
			matches := banre.FindStringSubmatch(message.Message)
			banList = fmt.Sprintf("%s\n%s", banList, matches[1])
			banUser(matches[1], message.Channel)
			commandMessage := fmt.Sprintf("Added %s to the banned users list.", matches[1])
			IRCCLIENT.Say(target, commandMessage)
		} else if filterre.MatchString(message.Message) {
			target := message.Channel
			matches := filterre.FindStringSubmatch(message.Message)
			banFilters = fmt.Sprintf("%s\n%s", banFilters, matches[1])
			if newFilters != "" {
				newFilters = fmt.Sprintf("%s\n%s", newFilters, matches[1])
			} else {
				newFilters = fmt.Sprintf("%s", matches[1])
			}
			commandMessage := fmt.Sprintf("Added %s to the filter list.", matches[1])
			IRCCLIENT.Say(target, commandMessage)
		}
	})

	//TODO -> move banning to ggo channel
	IRCCLIENT.OnUserStateMessage(func(message twitch.UserStateMessage) {
		zap.S().Infof("User %s joined the channel", message.User.Name)
		if message.User.Name != "" {
			if checkIfNameFiltered(message.User.Name) {
				banUser(message.User.Name, message.Channel)
			}
		}
		if message.User.DisplayName != "" {
			if checkIfNameFiltered(message.User.DisplayName) {
				banUser(message.User.DisplayName, message.Channel)
			}
		}
	})

	err := IRCCLIENT.Connect()
	if err != nil {
		zap.S().Errorf("Error connecting twitch IRCCLIENT: %v", err)
		panic(err)
	}
}
