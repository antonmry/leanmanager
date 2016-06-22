// Copyright © 2016 leanmanager
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slackbot

import (
	//"encoding/json"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/antonmry/leanmanager/api"
	"golang.org/x/net/websocket"
)

var (
	dailyChannel         = make(chan Message)
	waitingMessage int32 = 0
	channelId      string
	teamId         string
	apiserverUrl   string
)

func LaunchSlackbot(slackTokenArg, teamIdArg, apiserverHostArg string, apiserverPortArg int) {

	// Global variables
	teamId = teamIdArg
	apiserverUrl = "http://" + apiserverHostArg + ":" + strconv.Itoa(apiserverPortArg)

	// Open connection with Slack
	ws, id, err := slackConnect(slackTokenArg)
	if err != nil {
		log.Fatalf("Error connecting to Slack, check your token and Internet connection: %v", err)
	}

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
			log.Printf("Slack: error receiving message, %s\n", err)
			continue
		} else {
			go func(m Message) {
				manageMessage(m, id, ws)
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

func manageMessage(m Message, id string, ws *websocket.Conn) {

	// Only for debug
	//log.Println(m)

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

		newChannel := api.Channel{Id: channelId,
			Name:   channelId,
			TeamId: teamId}

		var buf bytes.Buffer
		err := json.NewEncoder(&buf).Encode(&newChannel)
		resp, err := http.Post(apiserverUrl+"/channels",
			"application/json", &buf)

		defer resp.Body.Close()

		if err != nil || resp.StatusCode != 201 {
			log.Printf("slackbot: error invoking API Server to save the channel: %v", err)
			return
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
		newMember := api.Member{m.Text[strings.Index(m.Text, ": add")+6:], m.Text[strings.Index(m.Text,
			": add")+6:], channelId, teamId}

		// DB persist
		var buf bytes.Buffer
		err := json.NewEncoder(&buf).Encode(&newMember)
		resp, err := http.Post(apiserverUrl+"/members",
			"application/json", &buf)

		defer resp.Body.Close()

		if err != nil || resp.StatusCode != 201 {
			log.Printf("slackbot: error invoking API Server to store new member %s in channel %s: %v",
				newMember.Name, channelId, err)
			return
		}

		message := &Message{0, "message", channelId, "Team member " + newMember.Name + " registered"}
		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slack: error adding member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: delete") {
		memberToBeDeleted := m.Text[strings.Index(m.Text, ": delete")+9:]

		clientAPI := &http.Client{}

		delMemberReq, _ := http.NewRequest("DELETE", apiserverUrl+"/members/"+
			channelId+"/"+memberToBeDeleted, nil)

		resp, err := clientAPI.Do(delMemberReq)
		defer resp.Body.Close()

		if err != nil || resp.StatusCode != 200 {
			log.Printf("slackbot: error invoking API Server to delete member %s in channel %s: %s",
				memberToBeDeleted, channelId, err)
			message := &Message{0, "message", channelId, "error deleting member"}
			sendMessage(ws, *message)
			return
		}

		message := &Message{0, "message", channelId, "Team member " + memberToBeDeleted + " deleted"}
		if err := sendMessage(ws, *message); err != nil {
			log.Printf("Slack: error deleting member in channel %s: %s\n", channelId, err)
		}
		return
	}

	if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">: list") {

		var RegisteredMemberMessage string
		var teamMembers []api.Member

		resp, err := http.Get(apiserverUrl + "/members/" + channelId)
		defer resp.Body.Close()

		if err != nil || resp.StatusCode != 200 {
			log.Printf("slackbot: error invoking API Server to retrieve members of channel: %v", err)
			return
		}

		buf, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(buf, &teamMembers)

		if err != nil {
			log.Printf("slackbot: error parsing API Server response with members of channel: %v", err)
			return
		}

		for i := 0; i < len(teamMembers); i++ {
			// FIXME: minor perfomance but it should be a buffer, not a string
			RegisteredMemberMessage += teamMembers[i].Name + ", "
		}

		if RegisteredMemberMessage == "" {
			RegisteredMemberMessage = "There are no members registered yet. Type " +
				"`@leanmanager: add @username` to add the first one"
		} else {
			RegisteredMemberMessage = "Members registered for the next Daily Sprint: " +
				RegisteredMemberMessage[:len(RegisteredMemberMessage)-2]
		}

		message := &Message{0, "message", channelId, RegisteredMemberMessage}
		if err := sendMessage(ws, *message); err != nil {
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

		var teamMembers []api.Member
		var dailyMeetingMessage *Message
		var messageReceived Message

		resp, err := http.Get(apiserverUrl + "/members/" + channelId)
		defer resp.Body.Close()

		if err != nil || resp.StatusCode != 200 {
			log.Printf("slackbot: error invoking API Server to retrieve members of channel: %v", err)
			return
		}

		buf, err := ioutil.ReadAll(resp.Body)
		json.Unmarshal(buf, &teamMembers)

		if err != nil {
			log.Printf("slackbot: error parsing API Server response with members of channel: %v", err)
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
				log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}

			atomic.StoreInt32(&waitingMessage, 1)
			for {
				messageReceived = <-dailyChannel
				if messageReceived.Type == "message" &&
					(strings.HasPrefix(messageReceived.Text, "<@"+id+">: no") ||
						strings.HasPrefix(messageReceived.Text, "<@"+id+">: yes")) {
					break
				} else {
					//FIXME: do we should manage this?, how?, also in the following messages
				}
			}
			atomic.StoreInt32(&waitingMessage, 0)

			if messageReceived.Type == "message" &&
				strings.HasPrefix(messageReceived.Text, "<@"+id+">: no") {
				dailyMeetingMessage = &Message{0, "message", channelId, "Ok, you can do it later,  " +
					"just type `@leanmanager resume` before the end of the day"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
				continue
			}

			if messageReceived.Type == "message" &&
				strings.HasPrefix(messageReceived.Text, "<@"+id+">: yes") {
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].Name +
					", what did you do yesterday?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				log.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
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
			log.Printf("Slack: %s: received message from messageChanel: %s\n", channelId, messageReceived)

			if messageReceived.Type == "message" && strings.HasPrefix(messageReceived.Text, "<@"+id+">:") {
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].Name +
					", what will you do today?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				log.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
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
			log.Printf("Slack: %s: received message from messageChanel: %s\n", channelId, messageReceived)

			if messageReceived.Type == "message" && strings.HasPrefix(messageReceived.Text, "<@"+id+">:") {
				dailyMeetingMessage = &Message{0, "message", channelId, teamMembers[i].Name +
					", are there any impediments in your way?. Please, start with `@leanmanager: `"}
				if err := sendMessage(ws, *dailyMeetingMessage); err != nil {
					log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
				}
			} else {
				log.Printf("Slack: unexpected flow in channel %s: %s\n", channelId, err)
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
				log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}
		}

		endDailyMeetingMessage := &Message{0, "message", channelId, "Daily Meeting done :tada: Have a " +
			"great day!"}
		if err := sendMessage(ws, *endDailyMeetingMessage); err != nil {
			log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
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
				log.Printf("Slack: error sending message to channel %s: %s\n", channelId, err)
			}
			log.Printf("Slack: message not understood\n")
		}
		return
	}
}
