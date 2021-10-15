package main

import (
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

/* General */

func safelistUser(user string) {
	zap.S().Infof("Safelisting user %s for this stream.", user)
	if userSafelist != "" {
		userSafelist = fmt.Sprintf("%s\n%s", userSafelist, user)
	} else {
		userSafelist = fmt.Sprintf("%s", user)
	}
}

func banUser() {

}

func checkUserList(users []string) {

}

/* Name Parsing */

// checkUserBanStatus checks if a user's name is on the ban list, or matches a
// ban filter. It returns true if the name matches, false otherwise.
func checkUserBanStatus(name string) bool {
	if banList == "" {
		zap.S().Info("No banned usernames, if this is expected, please ignore.")
	} else if strings.Contains(banList, name) {
		zap.S().Infof("Found the name %s in the ban lists.", name)
		return true
	}
	if banFilters == "" {
		zap.S().Info("No filters found, if this is expected, please ignore.")
	} else {
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
	}
	return false
}
