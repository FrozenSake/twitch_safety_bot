package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"os/signal"
	"time"

	"github.com/gempir/go-twitch-irc/v2"
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

var CLIENT *twitch.Client

var (
	banLog     string
	banList    string
	banFilters string
	newFilters string
	userWhitelist string
)

/*

Read Files
Create Files

Connect to twitch IRC
read user list in twitch
consume channel events in twitch

Call twitch web API
get user id
block user

compare lists
compare list against regex

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
	userlist, err := CLIENT.Userlist(channelName)
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
			banMessage := fmt.Sprintf("/ban %s", val)
			CLIENT.Say(channelName, banMessage)
			if banLog != "" {
				banLog = fmt.Sprintf("%s\n%s", banLog, val)
			} else {
				banLog = fmt.Sprintf("%s", val)
			}
		} else {
			zap.S().Infof("Whitelisting user %s for this stream.", val)
			if userWhitelist != "" {
				userWhitelist = fmt.Sprintf("%s\n%s", userWhitelist, val)
			} else {
				userWhitelist = fmt.Sprintf("%s", val)
			}
		}
	}
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
			zap.S().Infof("Adding ^ to filter: %s", filter)
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
			case <- ticker.C:
				getAndCheckUserList("hikthur")
			case <- userlistChan:
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

	zap.S().Infof("Creating Twitch Client: %v", username)
	CLIENT = twitch.NewClient(username, oauth)

	zap.S().Info("Prepare channels")
	for _, channelName := range targets {
		channelName = strings.ToLower(channelName)
		zap.S().Infof("Joining channel %v", channelName)
		CLIENT.Join(channelName)

		bc := broadcaster{name: channelName, connected: true}
		channelsStatus[channelName] = bc
	}

	//TODO -> move banning to go channel
	CLIENT.OnPrivateMessage(func(message twitch.PrivateMessage) {
		zap.S().Infof("User %s joined the channel", message.User.Name)
		if len(message.User.Badges) == 0 && !strings.Contains(userWhitelist, message.User.Name) {
			zap.S().Info("No badges")
			if checkIfNameFiltered(message.User.Name) {
				banMessage := fmt.Sprintf("/ban %s", message.User.Name)
				CLIENT.Say(message.Channel, banMessage)
				if banLog != "" {
					banLog = fmt.Sprintf("%s\n%s", banLog, message.User.Name)
				} else {
					banLog = fmt.Sprintf("%s", message.User.Name)
				}
			} else {
				zap.S().Infof("Whitelisting user %s for this stream.", message.User.Name)
				if userWhitelist != "" {
					userWhitelist = fmt.Sprintf("%s\n%s", userWhitelist, message.User.Name)
				} else {
					userWhitelist = fmt.Sprintf("%s", message.User.Name)
				}
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
			if banLog != "" {
				banLog = fmt.Sprintf("%s\n%s", banLog, matches[1])
			} else {
				banLog = fmt.Sprintf("%s", matches[1])
			}
			commandMessage := fmt.Sprintf("Added %s to the banned users list.", matches[1])
			CLIENT.Say(target, commandMessage)
			banMessage := fmt.Sprintf("/ban %s", matches[1])
			CLIENT.Say(target, banMessage)
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
			CLIENT.Say(target, commandMessage)
		}
	})

	//TODO -> move banning to ggo channel
	CLIENT.OnUserStateMessage(func(message twitch.UserStateMessage) {
		zap.S().Infof("User %s joined the channel", message.User.Name)
		if message.User.Name != "" {
			if checkIfNameFiltered(message.User.Name) {
				banMessage := fmt.Sprintf("/ban %s", message.User.Name)
				CLIENT.Say(message.Channel, banMessage)
				if banLog != "" {
					banLog = fmt.Sprintf("%s\n%s", banLog, message.User.Name)
				} else {
					banLog = fmt.Sprintf("%s", message.User.Name)
				}
			}
		}
		if message.User.DisplayName != "" {
			if checkIfNameFiltered(message.User.DisplayName) {
				banMessage := fmt.Sprintf("/ban %s", message.User.DisplayName)
				CLIENT.Say(message.Channel, banMessage)
				if banLog != "" {
					banLog = fmt.Sprintf("%s\n%s", banLog, message.User.DisplayName)
				} else {
					banLog = fmt.Sprintf("%s", message.User.DisplayName)
				}
			}
		}
	})

	err := CLIENT.Connect()
	if err != nil {
		zap.S().Errorf("Error connecting twitch client: %v", err)
		panic(err)
	}
}
