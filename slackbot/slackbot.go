// Package slackbot provides all the leanmanager logic for the Slack bot
package slackbot

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

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

	println("DEBUG: Periodic launcher started")
	//TODO: we have to update once the daily is launched, to avoid problems an infinite loop
	channelsDailyMap.Lock()
	defer channelsDailyMap.Unlock()

	for k, v := range channelsDailyMap.d {
		t := time.Now()
		// Firt, check there are than 23 hours since the last one
		// TODO: we have to stablish a limit for the Daily and a Timeout
		if !v.lastDaily.IsZero() && v.lastDaily.Sub(t).Hours() < 23 {
			continue
		}

		// Then check if today is a daily meeting day
		fmt.Printf("DEBUG: Today is: %s\n", t.Weekday())
		i := sort.SearchStrings(v.days, string(t.Weekday()))
		// FIXME: we have to compare Weekdays, not strings!!
		if !(i < len(v.days) && v.days[i] == t.Weekday().String()) {
			continue
		}

		// Check if it's time to start the meeting
		tReference := time.Date(0, 1, 1, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		if v.startTime.Sub(tReference) > 0 {
			fmt.Printf("DEBUG: time for the next one: %2.2f\n", v.startTime.Sub(tReference).Seconds())
			continue
		}

		// TODO: implement team availability check
		teamAvailability := true

		if !v.limitTime.IsZero() && v.limitTime.Sub(tReference) > 0 && !teamAvailability {
			fmt.Printf("DEBUG: time for the next one: %2.2f\n when connected", v.limitTime.Sub(tReference).Seconds())
			continue
		}
		fmt.Println("DEBUG: start meeting")

		m := &Message{
			ID:      0,
			Type:    "message",
			Channel: k,
			Text:    "",
		}
		// FIXME: what happen if we launch two daily here at the same time? m is going to be share!
		go func() {
			manageStartDaily(ws, m)
		}()
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
	case m.isStartDaily(botID):
		manageStartDaily(ws, &m)
	case m.isResumeDailyMsj(botID):
		manageResumeDaily(ws, &m)
	case m.isInfoDaily(botID):
		manageInfoDaily(ws, &m)
	case m.isScheduleDaily(botID):
		manageScheduleDaily(ws, &m)
	case m.isCommand(botID):
		manageUnderstoodCommand(ws, &m)
		log.Printf("slackbot: bot %s has received an understood message", botID)
	case isExpectedMessage(&m):
		manageExpectedMessage(ws, &m)
	}
}
