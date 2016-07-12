// Package slackbot provides all the leanmanager logic for the Slack bot
package slackbot

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/antonmry/leanmanager/api"

	"golang.org/x/net/websocket"
)

var (
	teamID       string
	apiserverURL string
)

// LaunchSlackbot starts the Slackbot connecting to Slack and starting to process messages
func LaunchSlackbot(slackTokenArg, teamIDArg, apiserverHostArg string, apiserverPortArg int) {

	// Global variables
	teamID = teamIDArg
	apiserverURL = "http://" + apiserverHostArg + ":" + strconv.Itoa(apiserverPortArg)

	// Open connection with Slack
	ws, botID, err := slackConnect(slackTokenArg)
	if err != nil {
		log.Fatalf("Error connecting to Slack, check your token and Internet connection: %v", err)
	}

	log.Println("slackbot: bot connected")

	resp, err := http.Get(apiserverURL + "/dailymeetings/" + teamID)
	defer resp.Body.Close()

	if err != nil || resp.StatusCode != 200 {
		log.Fatalf("slackbot: error invoking API Server to retrieve daily meetings"+
			" for bot: %v", err)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("slackbot: error parsing API Server response with "+
			"daily meetings of the bot: %v", err)
	}
	var teamDailyMeetings []api.DailyMeeting
	json.Unmarshal(buf, &teamDailyMeetings)

	channelsDailyMap.Lock()
	for _, t := range teamDailyMeetings {
		// TODO: key should be a boolean, not ChannelID
		channelsDailyMap.d[t.ChannelID] = api.DailyMeeting{
			ChannelID: t.ChannelID,
			LastDaily: t.LastDaily,
			StartTime: t.StartTime,
			LimitTime: t.LimitTime,
			Days:      t.Days,
		}
	}
	channelsDailyMap.Unlock()

	// Scheduled tasks (daily launching)
	t := time.NewTicker(60 * time.Second)
	go func() {
		for {
			launchScheduledTasks(ws)
			<-t.C
		}
	}()

	// Message processing
	for {
		if m, err := receiveMessage(ws); err != nil {
			log.Printf("slackbot: error receiving message, %s\n", err)
			continue
		} else {
			go func(m Message) {
				manageMessage(m, botID, ws)
			}(m)
		}
	}
}

func launchScheduledTasks(ws *websocket.Conn) {

	channelsDailyMap.Lock()
	defer channelsDailyMap.Unlock()

	for _, v := range channelsDailyMap.d {
		t := time.Now()
		// Firt, check there are than 12 hours since the last one
		if !v.LastDaily.IsZero() && v.LastDaily.Sub(t).Hours() < 12 {
			continue
		}

		// Then check if today is a daily meeting day
		found := false
		for _, d := range v.Days {
			if d == t.Weekday() {
				found = true
			}
		}

		if !found {
			continue
		}

		// Check if it's time to start the meeting
		tReference := time.Date(0, 1, 1, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		if v.StartTime.Sub(tReference) > 0 {
			continue
		}

		teamAvailability := true

		if !v.LimitTime.IsZero() && v.LimitTime.Sub(tReference) > 0 && !teamAvailability {
			continue
		}

		m := Message{
			ID:      0,
			Type:    "message",
			Channel: v.ChannelID,
			Text:    "",
		}
		go func(m Message) {
			manageStartDaily(ws, &m)
		}(m)
	}
}

func manageMessage(m Message, botID string, ws *websocket.Conn) {

	if m.getChannelID() == "" {
		return
	}

	switch {
	case m.isInitialMsj(botID):
		manageHello(ws, &m)
	case m.isAddMemberDailyMsj(botID):
		manageAddMember(ws, &m)
	case m.isDeleteMemberDailyMsj(botID):
		manageDelMember(ws, &m)
	case m.isListMembersDailyMsj(botID):
		manageListMembers(ws, &m)
	case m.isStartDailyMsj(botID):
		manageStartDaily(ws, &m)
	case m.isResumeDailyMsj(botID):
		manageResumeDaily(ws, &m)
	case m.isInfoDailyMsj(botID):
		manageInfoDaily(ws, &m)
	case m.isScheduleDailyMsj(botID):
		manageScheduleDaily(ws, &m)
	case m.isAddReplyDailyMsj(botID):
		manageAddReplyDaily(ws, &m)
	case m.isDeleteReplyDailyMsj(botID):
		//TODO: manageDeleteReplyDaily(ws, &m)
	case m.isHelpMsj(botID):
		manageHelp(ws, &m)
	case m.isCommand(botID):
		manageUnderstoodCommand(ws, &m)
		log.Printf("slackbot: bot %s has received an understood message", botID)
	case isExpectedMessage(&m):
		manageExpectedMessage(ws, &m)
	}
}
