package main

import (
	"fmt"

	"github.com/nicklaw5/helix"
	"go.uber.org/zap"
)

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

func setRateLimitWait(rate int) {
	rateLimitWait = rate
}
