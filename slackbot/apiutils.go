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

func addDailyMeeting(daily *api.DailyMeeting, botID string) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(&daily)
	resp, err := http.Post(apiserverURL+"/dailymeetings",
		"application/json", &buf)

	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 201 {
		return fmt.Errorf("apiutils: error invoking API Server to store new daily meeting for channel %s: %v",
			daily.ChannelID, err)
	}

	return nil
}

// FIXME: it should be a pointer, not copy the slice!
func listDailyMeetings(botID string) (teamDailyMeetings []api.DailyMeeting, err error) {
	resp, err := http.Get(apiserverURL + "/dailymeetings/")
	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("slackbot: error invoking API Server to retrieve daily members "+
			" of bot: %v", err)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slackbot: error parsing API Server response with "+
			"daily meetings of bot: %v", err)
	}

	json.Unmarshal(buf, &teamDailyMeetings)

	return teamDailyMeetings, nil
}

func addPredefinedReply(reply *api.PredefinedDailyReply) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(&reply)
	resp, err := http.Post(apiserverURL+"/replies",
		"application/json", &buf)

	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 201 {
		return fmt.Errorf("apiutils: error invoking API Server to store new predefined reply for channel %s: %v",
			reply.ChannelID, err)
	}

	return nil
}

func listPredefinedReplies(channelID string) (predefinedReplies *[]api.PredefinedDailyReply, err error) {
	resp, err := http.Get(apiserverURL + "/replies/" + channelID)
	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("slackbot: error invoking API Server to retrieve predefined replies"+
			" of bot: %v", err)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("slackbot: error parsing API Server response with "+
			"daily meetings of bot: %v", err)
	}

	json.Unmarshal(buf, &predefinedReplies)

	return predefinedReplies, nil
}

func addTeamMember(member *api.Member) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(&member)
	resp, err := http.Post(apiserverURL+"/members",
		"application/json", &buf)

	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 201 {
		return fmt.Errorf("apiutils: error invoking API Server to store new member %s in channel %s: %v",
			member.Name, member.ChannelID, err)
	}

	return nil

}

func delTeamMember(member *api.Member) error {
	clientAPI := &http.Client{}

	delMemberReq, _ := http.NewRequest("DELETE", apiserverURL+"/members/"+
		member.ChannelID+"/"+member.ID, nil)

	resp, err := clientAPI.Do(delMemberReq)
	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("apiutils: error invoking API Server to delete member %s in channel %s: %v",
			member.ID, member.ChannelID, err)
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
