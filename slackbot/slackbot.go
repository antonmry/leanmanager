// Package slackbot provides all the leanmanager logic for the Slack bot
package slackbot

import (
	"log"
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
	t := time.NewTicker(600 * time.Second)
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
	log.Printf("slackbot: periodic message sent\n")
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
