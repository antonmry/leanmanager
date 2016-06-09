package main

import (
	"fmt"
	"github.com/boltdb/bolt"
	"golang.org/x/net/websocket"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	//TODO: these must be read as ENV variable (docker)
	slackToken = "xoxb-47966556151-mh3PMjIpzo1vUYImrqFAfCw2"
	pathDb     = "/tmp"
)

type memberRecord struct {
	ID int
	name string
}

var channelId string
var registeredMembersStorage = make(map[memberRecord]string)
var registeredMembersMutex = sync.RWMutex{}
var dailyChannel = make(chan Message)
var waitingMessage int32 = 0

// TODO: check sync.WaitGroup https://blog.golang.org/pipelines
// TODO: timeouts: https://gobyexample.com/timeouts
// TODO: awesome thread sync https://groups.google.com/forum/#!topic/golang-nuts/eIqkhXh9PLg
// https://gobyexample.com/atomic-counters

func main() {
	// Open DB
	// TODO: url path must be provided as ENV variable
	db, err := bolt.Open(pathDb+"/slackbot.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Open connection with Slack
	ws, id := slackConnect(slackToken)
	log.Println("Slack: bot connected")

	// Scheduled tasks
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
			//TODO: if error, we should reconnect!!
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
		newMember := memberRecord{0, m.Text[strings.Index(m.Text, ": add")+6:]}
		// TODO: check if newMember is member of the channel
		// TODO: allow to add many members with one command
		// TODO: check if we have the channel bucket before instert in dB!!

		// DB persist
		if err := storeMember(db, &newMember, channelId); err != nil {
			log.Printf("Slack: error storing new member in channel %s: %s\n", channelId, err)
		}

		message := &Message{0, "message", channelId, "Team member " + newMember.name + " registered"}
		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slack: error adding member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: delete") {
		memberToBeDeleted := memberRecord{0, m.Text[strings.Index(m.Text, ": free")+7:]}

		err := deleteMember(db, &memberToBeDeleted, channelId)

		if serr, ok := err.(*NotMemberFoundError); ok {
			message := &Message{0, "message", channelId, fmt.Sprintf("%s", serr)}
			sendMessage(ws, *message)
			return
		}

		if err != nil {
			log.Printf("Slack: error deleting member in channel %s: %s\n", channelId, err)
		}

		message := &Message{0, "message", channelId, "Team member " + memberToBeDeleted.name + " deleted"}
		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slack: error deleting member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: list") {
		registeredMembersMutex.Lock()
		RegisteredMemberMessage := ""

		for k, v := range registeredMembersStorage {
			if v == channelId {
				RegisteredMemberMessage += k.name + ", "
			}
		}
		registeredMembersMutex.Unlock()

		if RegisteredMemberMessage == "" {
			RegisteredMemberMessage = "There is no members registered yet. Type " +
				"`@leanmanager: add @username` to add the first one"
		} else {
			RegisteredMemberMessage = "Members registered for the next Daily Sprint: " +
				RegisteredMemberMessage[:len(RegisteredMemberMessage)-2]
		}

		message := &Message{0, "message", channelId, RegisteredMemberMessage}
		err := sendMessage(ws, *message)
		if err != nil {
			fmt.Printf("Slack: error listing member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: start") {
		message := &Message{0, "message", channelId, "Hi @everyone! Let's start the Daily Meeting :mega:"}
		err := sendMessage(ws, *message)
		if err != nil {
			log.Printf("Slack: error starting the daily in channel %s: %s\n", channelId, err)
		}

		// We don't want to block our storage during the Daily Meeting
		//registeredMembersMutex.Lock()
		//teamMembers := make([]memberRecord, 0)
		//for k, v := range registeredMembersStorage {
		//	if v == channelId {
		//		teamMembers = append(teamMembers, k)
		//	}
		//}
		//registeredMembersMutex.Unlock()
		teamMembers, err := getTeamMembers(db, channelId)
		if err != nil {
			log.Printf("Slack: error retrieving members in channel %s: %s\n", channelId, err)
		}

		// FIXME: check if there are no members!!
		for i := range teamMembers {
			message := &Message{0, "message", channelId, "Hi " + teamMembers[i].name +
				"! Are you ready?. Type `@leanmanager: yes` or `@leanmanager: no`"}
			err := sendMessage(ws, *message)
			if err != nil {
				fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}

			var dailyMeetingMessage *Message
			var messageReceived Message

			// TODO: how we differ messages by channel and user?
			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					(strings.HasPrefix(messageReceived.Text, "<@"+id+">: no") ||
						strings.HasPrefix(messageReceived.Text, "<@"+id+">: yes")) {
					break
				} else {
					//TODO: we should manage this, also in the following messages
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
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].name +
					", what did you do yesterday?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				fmt.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
				return
			}

			// TODO: store message? waitingMessage atomic? timeOut?
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
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].name +
					", what will you do today?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				fmt.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
				return
			}

			// TODO: store message? waitingMessage atomic? timeOut?
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
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].name +
					", are there any impediments in your way?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					fmt.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				fmt.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
				return
			}

			// TODO: store message? waitingMessage atomic or channel? timeOut?
			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					strings.HasPrefix(messageReceived.Text, "<@"+id+">: ") {
					break
				}
			}
			atomic.StoreInt32(&waitingMessage, 0)

			dailyMeetingMessage = &Message{0, "message", channelId, "Thanks " + teamMembers[i].name}
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
		//TODO: schedule the daily meeting
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: help") {
		//TODO: show help commands
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
