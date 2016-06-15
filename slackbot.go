package main

import (
	"fmt"
	"github.com/boltdb/bolt"
	"golang.org/x/net/websocket"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

const (
	slackToken = "xoxb-47966556151-mh3PMjIpzo1vUYImrqFAfCw2"
	pathDb     = "/tmp"
	teamId     = "NetApps"
)

type memberRecord struct {
	Name string
}

var channelId string
var dailyChannel = make(chan Message)
var waitingMessage int32 = 0

// TODO: CONST must be read as ENV variable, create BASH script to launch
// TODO: how we differ messages by channel and user? channelID shouldn't be a global variable
// TODO: timeouts: https://gobyexample.com/timeouts
// TODO: avoid @leanmanager prefix when possible!
// TODO: move logic from this file to a new one
// TODO: check sync.WaitGroup https://blog.golang.org/pipelines
// TODO: move open DB to dbutils
// TODO: if error, we should reconnect!!
// TODO: check if newMember is member of the channel when added
// TODO: allow to add many members with one command
// TODO: schedule the daily meeting
// TODO: show help commands

func main() {
	// Open DB
	db, err := bolt.Open(pathDb+"/"+teamId+".db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Open connection with Slack
	ws, id := slackConnect(slackToken)
	log.Println("Slack: bot connected")

	// Scheduled tasks (timeouts and daily launching)
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
			fmt.Printf("Slack: error receiving message, %s\n", err)
			continue
		} else {
			go func(m Message) {
				manageMessage(m, id, ws, db)
			}(m)
		}
	}
}

func launchScheduledTasks(ws *websocket.Conn) {
	if channelId != "" {
		log.Printf("Slack: periodic message sent\n")
	} else {
		log.Printf("Slack: periodic call invoked but channel ID not defined yet\n")
	}
}

func manageMessage(m Message, id string, ws *websocket.Conn, db *bolt.DB) {

	// Only for debug
	fmt.Println(m)

	// FIXME: we should have some channel logic here, not a global variable :-(
	if m.Type == "message" && m.Channel != nil {
		channelId = m.Channel.(string)
	}

	// Message logic

	if (m.Type == "group_joined" || m.Type == "channel_joined" || m.Type == "message") && strings.HasPrefix(m.Text,
		"<@"+id+">: hello") {
		if m.Type == "group_joined" || m.Type == "channel_joined" {
			t := m.Channel.(map[string]interface{})
			for k, v := range t {
				if k == "id" {
					if str, ok := v.(string); ok {
						channelId = str
						break
					}
				}
			}
			log.Printf("Slack: group_joined with id %s \n", channelId)
		}

		if m.Type == "message" {
			channelId = m.Channel.(string)
			log.Printf("Slackbot: message start with id %s \n", channelId)
		}

		if err := createBucket(db, channelId); err != nil {
			log.Fatalf("Slackbot: error creating db: %s", err)
		}

		messageText := "Hello team! I'm here to help you with your daily meetings. To add members " +
			"to the daily meeting type `@leanmanager: add @username`, to setup the hour of the " +
			"daily meeting, type something like `@leanmanager: schedule monday tuesday friday 13:00`.\n" +
			"If you need help, just type `@leanmanager: help`"

		message := &Message{0, "message", channelId, messageText}

		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slackbot: error sending message to channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: add") {
		newMember := memberRecord{m.Text[strings.Index(m.Text, ": add")+6:]}

		// DB persist
		if err := storeMember(db, newMember, channelId); err != nil {
			log.Printf("Slack: error storing new member in channel %s: %s\n", channelId, err)
		}

		message := &Message{0, "message", channelId, "Team member " + newMember.Name + " registered"}
		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slack: error adding member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: delete") {
		memberToBeDeleted := memberRecord{m.Text[strings.Index(m.Text, ": delete")+9:]}

		err := deleteMember(db, &memberToBeDeleted, channelId)

		if serr, ok := err.(*NotMemberFoundError); ok {
			message := &Message{0, "message", channelId, fmt.Sprintf("%s", serr)}
			sendMessage(ws, *message)
			return
		}

		if err != nil {
			log.Printf("Slack: error deleting member in channel %s: %s\n", channelId, err)
		}

		message := &Message{0, "message", channelId, "Team member " + memberToBeDeleted.Name + " deleted"}
		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slack: error deleting member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: list") {

		var RegisteredMemberMessage string
		var teamMembers []memberRecord

		if err := getTeamMembers(db, channelId, &teamMembers); err != nil {
			log.Printf("Slack: error retrieving members in channel %s: %s\n", channelId, err)
		} else {
			for i := 0; i < len(teamMembers); i++ {
				RegisteredMemberMessage += teamMembers[i].Name + ", "
			}
		}

		if RegisteredMemberMessage == "" {
			RegisteredMemberMessage = "There are no members registered yet. Type " +
				"`@leanmanager: add @username` to add the first one"
		} else {
			RegisteredMemberMessage = "Members registered for the next Daily Sprint: " +
				RegisteredMemberMessage[:len(RegisteredMemberMessage)-2]
		}

		message := &Message{0, "message", channelId, RegisteredMemberMessage}
		err := sendMessage(ws, *message)
		if err != nil {
			log.Printf("Slack: error listing member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: start") {
		message := &Message{0, "message", channelId, "Hi @everyone! Let's start the Daily Meeting :mega:"}
		err := sendMessage(ws, *message)
		if err != nil {
			log.Printf("Slack: error starting the daily in channel %s: %s\n", channelId, err)
		}

		var teamMembers []memberRecord
		var dailyMeetingMessage *Message
		var messageReceived Message

		if err := getTeamMembers(db, channelId, &teamMembers); err != nil {
			log.Printf("Slack: error retrieving members in channel %s: %s\n", channelId, err)
			return
		}

		if len(teamMembers) == 0 {
			message := &Message{0, "message", channelId, "There are no members registered yet. Type " +
				"`@leanmanager: add @username` to add the first one"}
			err := sendMessage(ws, *message)
			if err != nil {
				log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}
			return
		}

		for i := 0; i < len(teamMembers); i++ {

			message := &Message{0, "message", channelId, "Hi " + teamMembers[i].Name +
				"! Are you ready?. Type `@leanmanager: yes` or `@leanmanager: no`"}
			err := sendMessage(ws, *message)
			if err != nil {
				fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}

			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					(strings.HasPrefix(messageReceived.Text, "<@"+id+">: no") ||
						strings.HasPrefix(messageReceived.Text, "<@"+id+">: yes")) {
					break
				} else {
					//FIXME: we should manage this?, also in the following messages
				}
			}
			atomic.StoreInt32(&waitingMessage, 0)

			if messageReceived.Type == "message" &&
				strings.HasPrefix(messageReceived.Text, "<@"+id+">: no") {
				dailyMeetingMessage = &Message{0, "message", channelId, "Ok, you can do it later,  " +
					"just type `@leanmanager resume` before the end of the day"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
				continue
			}

			if messageReceived.Type == "message" &&
				strings.HasPrefix(messageReceived.Text, "<@"+id+">: yes") {
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].Name +
					", what did you do yesterday?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				fmt.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
				return
			}

			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					strings.HasPrefix(messageReceived.Text, "<@"+id+">: ") {
					break
				}
			}
			atomic.StoreInt32(&waitingMessage, 0)

			//Debug
			fmt.Printf("Slack: %s: received message from messageChanel: %s\n", channelId, messageReceived)

			if messageReceived.Type == "message" && strings.HasPrefix(messageReceived.Text, "<@"+id+">:") {
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].Name +
					", what will you do today?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				fmt.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
				return
			}

			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					strings.HasPrefix(messageReceived.Text, "<@"+id+">: ") {
					break
				}
			}
			atomic.StoreInt32(&waitingMessage, 0)

			//Debug
			fmt.Printf("Slack: %s: received message from messageChanel: %s\n", channelId, messageReceived)

			if messageReceived.Type == "message" && strings.HasPrefix(messageReceived.Text, "<@"+id+">:") {
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].Name +
					", are there any impediments in your way?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				fmt.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
				return
			}

			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					strings.HasPrefix(messageReceived.Text, "<@"+id+">: ") {
					break
				}
			}
			atomic.StoreInt32(&waitingMessage, 0)

			dailyMeetingMessage = &Message{0, "message", channelId, "Thanks " + teamMembers[i].Name}
			if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
				fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}
		}

		endDailyMeetingMessage := &Message{0, "message", channelId, "Daily Meeting done :tada: Have a " +
			"great day!"}
		if err := sendMessage(ws, *endDailyMeetingMessage); err != nil {
			fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
		}

		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: schedule") {
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: help") {
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">") {
		if atomic.LoadInt32(&waitingMessage) != 0 {
			dailyChannel <- m
		} else {
			message := &Message{0, "message", channelId, ":interrobang:"}

			if err := sendMessage(ws, *message); err != nil {
				fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}
			fmt.Printf("Slack: message not understood\n")
		}
		return
	}
}
