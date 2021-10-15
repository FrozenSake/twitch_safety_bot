package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v2"
	"github.com/nicklaw5/helix"
	"github.com/spf13/viper"

	"go.uber.org/zap"
)

type broadcaster struct {
	name      string
	connected bool
}

var (
	banLog       string
	banList      string
	banFilters   string
	newFilters   string
	userSafelist string
)

var rateLimitWait int // Number of milliseconds to wait to keep rate limit.

/*

Use userid and block user
	need to queue it up so we don't hit rate limits
	channel?


Setup local HTTP server as part of bot, for oauth and callbacks
localhost/auth  --> handler for oauth2 flow
ip:443/follow   --> handler for follow event webhook

*/

/* Clients */

const oauthForm = "oauth:"

var IRCCLIENT *twitch.Client
var HELIXCLIENT *helix.Client

func OauthCheck(oauth string) {
	zap.S().Debug("Checking OAuth Format")
	if oauth[:6] != oauthForm {
		zap.S().Debug("Fixing OAuth Format")
		oauth = oauthForm + oauth
	}
}

func shutdown() {
	//shutdown funcs
	zap.S().Info("Twitch safety bot shut down complete.")
	os.Exit(0)
}

/* Run */

func main() {
	sugar, _ := zap.NewDevelopment()
	defer sugar.Sync()
	zap.ReplaceGlobals(sugar)
	zap.S().Info("Safety Bot starting up.")

	/* Interrupt Handler go routine */
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)
	go func() {
		<-interruptChan
		zap.S().Info("Shutdown request received.")
		shutdown()
	}()

	/* Periodic user check go routine*/
	ticker := time.NewTicker(60 * time.Second)
	userCheckTickerChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				users := getUserList("hikthur")
				checkUserList(users)
			case <-userCheckTickerChan:
				ticker.Stop()
				return
			}
		}
	}()
	defer close(userCheckTickerChan)

	/* Ban go routine */
	bansChan := make(chan string)
	setRateLimitWait(1) // Called to ensure there's no panic if the channel activates early
	go func() {
		var bannedUser string
		for {
			select {
			case bansChan <- bannedUser:
				banIRCUser(bannedUser, "hikthur")
				time.Sleep(time.Duration(rateLimitWait) * time.Millisecond)
			}
		}
	}()

	loadConfig()
	username := viper.GetString("username")
	oauth := viper.GetString("oauth")
	channels := viper.GetString("channels")
	targets := strings.Split(channels, ",")

	OauthCheck(oauth)

	// Define a regex object
	filterre := regexp.MustCompile("^!FilterAdd (?P<filter>\\S*)")
	banre := regexp.MustCompile("^!BanAdd (?P<ban>\\S*)")

	/* Prepare the lists, set per-session list to "" */
	banList, banFilters = prepareLists()
	userSafelist = ""
	banLog = ""
	newFilters = ""

	zap.S().Infof("Creating Twitch IRCCLIENT: %v", username)
	IRCCLIENT = twitch.NewClient(username, oauth)
	joinChannels(targets)

	IRCCLIENT.OnPrivateMessage(func(message twitch.PrivateMessage) {
		zap.S().Debugf("User %s sent a messagein the channel", message.User.Name)
		if len(message.User.Badges) == 0 && !strings.Contains(userSafelist, message.User.Name) {
			zap.S().Debug("No badges")
			if checkUserBanStatus(message.User.Name) {
				bansChan <- message.User.Name
			} else {
				safelistUser(message.User.Name)
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
			banIRCUser(matches[1], message.Channel)
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
			if checkUserBanStatus(message.User.Name) {
				banIRCUser(message.User.Name, message.Channel)
			}
		}
		if message.User.DisplayName != "" {
			if checkUserBanStatus(message.User.DisplayName) {
				banIRCUser(message.User.DisplayName, message.Channel)
			}
		}
	})

	err := IRCCLIENT.Connect()
	if err != nil {
		zap.S().Errorf("Error connecting twitch IRCCLIENT: %v", err)
		panic(err)
	}
}
