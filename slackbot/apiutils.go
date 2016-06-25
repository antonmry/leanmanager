// Package slackbot provides all the leanmanager logic for the Slack bot
package slackbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/antonmry/leanmanager/api"
)

func storeChannel(c *api.Channel) error {

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(&c)
	resp, err := http.Post(apiserverURL+"/channels",
		"application/json", &buf)

	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 201 {
		return fmt.Errorf("apiutils: error invoking API Server to save the channel: %v", err)
	}

	return nil
}

func addTeamMember(member *api.Member) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(&member)
	resp, err := http.Post(apiserverURL+"/members",
		"application/json", &buf)

	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 201 {
		return fmt.Errorf("apiutils: error invoking API Server to store new member %s in channel %s: %v",
			member.Name, member.ChannelId, err)
	}

	return nil

}

func delTeamMember(member *api.Member) error {
	clientAPI := &http.Client{}

	delMemberReq, _ := http.NewRequest("DELETE", apiserverURL+"/members/"+
		member.ChannelId+"/"+member.Id, nil)

	resp, err := clientAPI.Do(delMemberReq)
	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("apiutils: error invoking API Server to delete member %s in channel %s: %v",
			member.Id, member.ChannelId, err)
	}

	return nil
}

func listTeamMembers(channelID string) (teamMembers []api.Member, err error) {
	resp, err := http.Get(apiserverURL + "/members/" + channelID)
	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("slackbot: error invoking API Server to retrieve members "+
			" of channel: %v", err)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slackbot: error parsing API Server response with "+
			"members of channel: %v", err)
	}

	json.Unmarshal(buf, &teamMembers)

	return teamMembers, nil
}
