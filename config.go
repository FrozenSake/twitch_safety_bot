package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	banListsLoc   = "./lists"
	banFiltersLoc = "./filters"
	banLogsLoc    = "./logs"
)

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

func loadConfig() {
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

/* Manage Ban List */

func addUserToBanList(user string) {
	if banLog != "" {
		banLog = fmt.Sprintf("%s\n%s", banLog, user)
	} else {
		banLog = fmt.Sprintf("%s", user)
	}
}

func readUserFromBanList(user string) bool {
	return false
}

func readBanList() {

}

/* Manage Safe List */

func addUserToSafeList(user string) {

}

func readUserFromSafeList() bool {
	return false
}

func readSafeList() {

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
