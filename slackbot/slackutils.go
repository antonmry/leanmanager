// Package slackbot provides all the leanmanager logic for the Slack bot
package slackbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/antonmry/leanmanager/api"

	"golang.org/x/net/websocket"
)

const (
	timeout int = 120
)

// Message represents the message received from Slack
type Message struct {
	ID      uint64      `json:"id"`
	Type    string      `json:"type"`
	User    string      `json:"user,omitempty"`
	Channel interface{} `json:"channel"`
	Text    string      `json:"text"`
}

// Channel represents the Slack Channel or Group where the bot is participating
type Channel struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	IsChannel string      `json:"is_channel"`
	Members   interface{} `json:"members"`
}

// Member defines the participants in the Channel with the bot
type Member string

type responseRtmStart struct {
	Ok    bool         `json:"ok"`
	Error string       `json:"error"`
	URL   string       `json:"url"`
	Self  responseSelf `json:"self"`
}

type responseSelf struct {
	ID string `json:"id"`
}

type atomicCounter struct {
	sync.Mutex
	i uint64
}

type dailyScheduler struct {
	sync.Mutex
	d map[string]api.DailyMeeting
}

type pendingMsjController struct {
	sync.Mutex
	p map[string]map[string]chan Message
}

var counter = atomicCounter{}

var channelsMap = pendingMsjController{
	p: make(map[string]map[string]chan Message),
}

var channelsDailyMap = dailyScheduler{
	d: make(map[string]api.DailyMeeting),
}

// Connection methods

func slackConnect(token string) (ws *websocket.Conn, botID string, err error) {
	wsURL, botID, err := slackInit(token)
	if err != nil {
		return nil, "", fmt.Errorf("slackutils: error initiating communication with slack: %s", err)
	}

	ws, err = websocket.Dial(wsURL, "", "https://api.slack.com/")
	if err != nil {
		return nil, "", fmt.Errorf("slackutils: error creating websocket: %s", err)
	}

	return ws, botID, nil
}

func slackInit(token string) (wsurl, id string, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("slackutils: error Get: %s", err)
	}
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("slackutils: error response from Get: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", "", fmt.Errorf("slackutils: error reading Body: %s", err)
	}

	var slackResp responseRtmStart
	err = json.Unmarshal(body, &slackResp)
	if err != nil {
		return "", "", fmt.Errorf("slackutils: error parsing Slack resp: %s", err)
	}

	if !slackResp.Ok {
		return "", "", fmt.Errorf("slackutils: error returning ko: %s", err)
	}

	wsurl = slackResp.URL
	id = slackResp.Self.ID
	return
}

func receiveMessage(ws *websocket.Conn) (m Message, err error) {
	err = websocket.JSON.Receive(ws, &m)
	return
}

// Messages management

func manageHello(ws *websocket.Conn, m *Message) {

	newChannel := api.Channel{
		ID:     m.getChannelID(),
		Name:   m.getChannelID(),
		TeamID: teamID}

	if err := storeChannel(&newChannel); err != nil {
		log.Printf("slackutils: API Server is failing storing channel %s: %s\n", m.getChannelID(), err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}

	if err := sendHelloMsj(ws, m.getChannelID()); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
	}
	return
}

func manageHelp(ws *websocket.Conn, m *Message) {

	if err := sendHelpMsj(ws, m.getChannelID()); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
	}
	return
}

func manageAddMember(ws *websocket.Conn, m *Message) {

	channelsMap.Lock()
	if channelsMap.p[m.getChannelID()] == nil {
		channelsMap.p[m.getChannelID()] = map[string]chan Message{}
	}
	if channelsMap.p[m.getChannelID()]["<@"+m.User+">"] == nil {
		channelsMap.p[m.getChannelID()]["<@"+m.User+">"] = make(chan Message)
		defer channelsMap.finishWaitingMember(m.getChannelID(), "<@"+m.User+">")
	}
	channelsMap.Unlock()

	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text:    "What members do you want to add to the Daily Meeting?",
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var messageReceived Message
	var users []string

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		users = messageReceived.getValidUserIDs()
		if len(users) > 0 {
			break
		}

		message.Text = ":scream: Type something like `@alice @bob and @carel` or `cancel`."
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

	}

	for _, u := range users {
		newMember := api.Member{
			ID:        u,
			Name:      u,
			ChannelID: m.getChannelID(),
			TeamID:    teamID,
		}

		if err := addTeamMember(&newMember); err != nil {
			log.Printf("slackutils: API Server is failing adding member to channel %s: %v", m.getChannelID(), err)
			_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		}

		message.Text = "Team member " + newMember.Name + " registered"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending msj to channel %s: %s\n", m.getChannelID(), err)
		}
	}
}

func manageDelMember(ws *websocket.Conn, m *Message) {

	channelsMap.Lock()
	if channelsMap.p[m.getChannelID()] == nil {
		channelsMap.p[m.getChannelID()] = map[string]chan Message{}
	}
	if channelsMap.p[m.getChannelID()]["<@"+m.User+">"] == nil {
		channelsMap.p[m.getChannelID()]["<@"+m.User+">"] = make(chan Message)
		defer channelsMap.finishWaitingMember(m.getChannelID(), "<@"+m.User+">")
	}
	channelsMap.Unlock()

	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text:    "Who isn't going to participate the Daily Meeting?",
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var messageReceived Message
	var users []string

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		users = messageReceived.getValidUserIDs()
		if len(users) > 0 {
			break
		}

		message.Text = ":scream: Type something like `@alice @bob and @carel` or `cancel`."
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

	}

	for _, u := range users {
		memberToBeDeleted := api.Member{
			ID:        u,
			Name:      u,
			ChannelID: m.getChannelID(),
			TeamID:    teamID,
		}

		if err := delTeamMember(&memberToBeDeleted); err != nil {
			log.Printf("slackutils: API Server is failing deleting member in channel %s: %v", m.getChannelID(), err)
			_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		}

		message.Text = "Team member " + memberToBeDeleted.Name + " unregistered"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending msj to channel %s: %s\n", m.getChannelID(), err)
		}
	}
}

func manageListMembers(ws *websocket.Conn, m *Message) {

	teamMembers, err := listTeamMembers(m.getChannelID())

	if err != nil {
		log.Printf("slackutils: error invoking API Server to retrieve members of channel: %v", err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}

	if len(teamMembers[:]) == 0 {
		if err := sendNotMembersRegisteredMsj(ws, m.getChannelID()); err != nil {
			log.Printf("slackutils: error listing member in channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}

	var b bytes.Buffer
	b.WriteString("Members registered for the next Daily Sprint: ")

	for i := 0; i < len(teamMembers[:]); i++ {
		b.WriteString(teamMembers[i].ID + ", ")
	}

	message := &Message{
		ID:      0,
		Type:    "message",
		Channel: m.getChannelID(),
		Text:    b.String()[:len(b.String())-2],
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error listing member in channel %s: %s\n", m.getChannelID(), err)
	}
	return

}

func manageStartDaily(ws *websocket.Conn, m *Message) {

	if err := sendStartDailyMsj(ws, m.getChannelID()); err != nil {
		log.Printf("slackutils: error starting the daily in channel %s: %s\n", m.getChannelID(), err)
	}

	teamMembers, err := listTeamMembers(m.getChannelID())

	if err != nil {
		log.Printf("slackutils: error invoking API Server to retrieve members of channel: %v", err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}

	if len(teamMembers[:]) == 0 {
		if err := sendNotMembersRegisteredMsj(ws, m.getChannelID()); err != nil {
			log.Printf("slackutils: error listing member in channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}

	channelsDailyMap.Lock()
	d := channelsDailyMap.d[m.getChannelID()]
	d.LastDaily = time.Now()
	channelsDailyMap.d[m.getChannelID()] = d
	channelsDailyMap.Unlock()

	if err := addDailyMeeting(&d, teamID); err != nil {
		log.Printf("slackutils: error invoking API Server to store LastDaily time: %v", err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}

	var messageReceived Message

	for i := 0; i < len(teamMembers[:]); i++ {

		message := &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text:    "Hi " + teamMembers[i].ID + "! Are you ready?.",
		}
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

		channelsMap.Lock()
		if channelsMap.p[m.getChannelID()] == nil {
			channelsMap.p[m.getChannelID()] = map[string]chan Message{}
		}
		if channelsMap.p[m.getChannelID()][teamMembers[i].ID] == nil {
			channelsMap.p[m.getChannelID()][teamMembers[i].ID] = make(chan Message)
			defer channelsMap.finishWaitingMember(m.getChannelID(), teamMembers[i].ID)
		}
		channelsMap.Unlock()

		var memberAvailable bool
		for {
			select {
			case <-time.After(time.Second * time.Duration(timeout)):
				memberAvailable = false
			case messageReceived = <-channelsMap.p[m.getChannelID()][teamMembers[i].ID]:
				memberAvailable = true
			}
			if !memberAvailable || (messageReceived.isYes() || messageReceived.isNo()) {
				break
			}
		}

		if messageReceived.isNo() || !memberAvailable {
			if err := sendNotAvailableMsj(ws, m.getChannelID()); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			continue
		}

		runDailyByMember(ws, m.getChannelID(), teamMembers[i].ID)
	}

	endDailyMeetingMessage := &Message{
		ID:      0,
		Type:    "message",
		Channel: m.getChannelID(),
		Text:    "Daily Meeting done :tada: Have a great day!",
	}
	if err := endDailyMeetingMessage.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	return
}

func manageResumeDaily(ws *websocket.Conn, m *Message) {
	if m.User != "" {
		runDailyByMember(ws, m.getChannelID(), "<@"+m.User+">")
		return
	}
}

func runDailyByMember(ws *websocket.Conn, channelID, memberID string) {
	// Initialization to wait for user responses
	channelsMap.Lock()
	if channelsMap.p[channelID] == nil {
		channelsMap.p[channelID] = map[string]chan Message{}
	}
	if channelsMap.p[channelID][memberID] == nil {
		channelsMap.p[channelID][memberID] = make(chan Message)
		defer channelsMap.finishWaitingMember(channelID, memberID)
	}
	channelsMap.Unlock()

	// Start the daily meeting
	dailyMeetingMessage := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text:    memberID + ", what did you do yesterday?",
	}
	if err := dailyMeetingMessage.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
		return
	}

	m := <-channelsMap.p[channelID][memberID]

	if r := m.getPredefinedReply(0); r != "" {
		dailyMeetingMessage.Text = r
		if err := dailyMeetingMessage.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
			return
		}
	}

	dailyMeetingMessage.Text = memberID + ", what will you do today?"
	if err := dailyMeetingMessage.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
		return
	}

	m = <-channelsMap.p[channelID][memberID]

	if r := m.getPredefinedReply(1); r != "" {
		dailyMeetingMessage.Text = r
		if err := dailyMeetingMessage.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
			return
		}
	}
	dailyMeetingMessage.Text = memberID + ", are there any impediments in your way?"
	if err := dailyMeetingMessage.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
		return
	}

	m = <-channelsMap.p[channelID][memberID]
	if r := m.getPredefinedReply(2); r != "" {
		dailyMeetingMessage.Text = r
		if err := dailyMeetingMessage.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
			return
		}
	}
	dailyMeetingMessage.Text = "Thanks " + memberID
	if err := dailyMeetingMessage.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", channelID, err)
		return
	}
}

func manageUnderstoodCommand(ws *websocket.Conn, m *Message) {
	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text:    ":interrobang:",
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}
}

func manageScheduleDaily(ws *websocket.Conn, m *Message) {

	channelsMap.Lock()
	if channelsMap.p[m.getChannelID()] == nil {
		channelsMap.p[m.getChannelID()] = map[string]chan Message{}
	}
	if channelsMap.p[m.getChannelID()]["<@"+m.User+">"] == nil {
		channelsMap.p[m.getChannelID()]["<@"+m.User+">"] = make(chan Message)
		defer channelsMap.finishWaitingMember(m.getChannelID(), "<@"+m.User+">")
	}
	channelsMap.Unlock()

	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text:    "What days of the week you would like to run the Daily meeting?",
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var messageReceived Message
	var doW []time.Weekday

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		doW = messageReceived.getValidDays()
		if len(doW) > 0 {
			break
		}

		message.Text = ":scream: Type something like `weekdays`, `monday tuesday wednesday` or `cancel`."
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

	}

	message.Text = "What time do you want to start the meeting? :clock2:"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var startHour string
	var startTime time.Time

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]

		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		var err error
		startHour = messageReceived.getValidHour()
		startTime, err = api.ConvertTime(startHour)

		if startHour != "" && err == nil {
			break
		}

		message.Text = ":scream: Type something like `13:00`, `08:00AM` or `cancel`."
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}
	}

	message.Text = "Do you want stablish a flexible time based in your team's members activity?"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]

		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		if messageReceived.isNo() {
			if err := storeScheduledTime(m.getChannelID(), time.Time{}, startTime, time.Time{}, doW); err != nil {
				sendUnexpectedProblemMsj(ws, m.getChannelID())
				return
			}
			manageInfoDaily(ws, m)
			return
		}

		if messageReceived.isYes() {
			break
		}

		message.Text = ":scream: Type something like `yes`, `no` or `cancel`."
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}
	}

	message.Text = "What time is the limit to start? :clock8:"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var limitHour string
	var limitTime time.Time

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]

		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		var err error
		limitHour = messageReceived.getValidHour()
		limitTime, err = api.ConvertTime(limitHour)

		if limitHour != "" && err == nil && !limitTime.Before(startTime) {
			break
		}

		if !limitTime.Before(startTime) {
			message.Text = ":scream: Type something like `13:00`, `08:00AM` or `cancel`."
		} else {
			message.Text = "Ok, it's not how you start, it's how you finish.. but you have to start first :stuck_out_tongue_closed_eyes:"
		}
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}
	}

	if err := storeScheduledTime(m.getChannelID(), time.Time{}, startTime, limitTime, doW); err != nil {
		sendUnexpectedProblemMsj(ws, m.getChannelID())
	}

	manageInfoDaily(ws, m)
}

func storeScheduledTime(channelID string, lastDaily, startTime, limitTime time.Time, doW []time.Weekday) error {

	channelsDailyMap.Lock()
	defer channelsDailyMap.Unlock()
	dailyToAdd := api.DailyMeeting{
		ChannelID: channelID,
		LastDaily: time.Time{},
		StartTime: startTime,
		LimitTime: limitTime,
		Days:      doW,
	}

	channelsDailyMap.d[channelID] = dailyToAdd

	return addDailyMeeting(&dailyToAdd, teamID)
}

func manageInfoDaily(ws *websocket.Conn, m *Message) {
	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text: "There is no Daily Meeting scheduled yet, type `@leanmanager: daily schedule` " +
			"to schedule your next Daily Meeting",
	}
	channelsDailyMap.Lock()

	if i, ok := channelsDailyMap.d[m.getChannelID()]; ok {

		var b bytes.Buffer
		b.WriteString("Daily Meeting scheduled on ")

		for _, w := range i.Days {
			b.WriteString(w.String() + ", ")
		}

		if i.LimitTime.IsZero() {
			message.Text = fmt.Sprintf(b.String()[:len(b.String())-2]+" at %02d:%02d",
				i.StartTime.Hour(), i.StartTime.Minute())
		} else {
			message.Text = fmt.Sprintf(b.String()[:len(b.String())-2]+" between %02d:%02d and %02d:%02d",
				i.StartTime.Hour(), i.StartTime.Minute(),
				i.LimitTime.Hour(), i.LimitTime.Minute())
		}
		if !i.LastDaily.IsZero() {
			message.Text += fmt.Sprintf("\nLast meeting done %2.2f hours ago", time.Since(i.LastDaily).Hours())
		}
	}

	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}
	channelsDailyMap.Unlock()
}

func manageAddReplyDaily(ws *websocket.Conn, m *Message) {

	channelsMap.Lock()
	if channelsMap.p[m.getChannelID()] == nil {
		channelsMap.p[m.getChannelID()] = map[string]chan Message{}
	}
	if channelsMap.p[m.getChannelID()]["<@"+m.User+">"] == nil {
		channelsMap.p[m.getChannelID()]["<@"+m.User+">"] = make(chan Message)
		defer channelsMap.finishWaitingMember(m.getChannelID(), "<@"+m.User+">")
	}
	channelsMap.Unlock()

	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text:    "To what question I should reply? First one, second one or last one?",
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var messageReceived Message
	var question int

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		var err error
		if question, err = messageReceived.getValidAnswer(); err == nil {
			break
		}

		message.Text = ":scream: Type something like `first one`, `second one`, `last one` or `cancel`."
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

	}

	message.Text = "What is the regular expression which matches the answer of the team member to that question?"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	var exp string

	for {
		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		var err error
		if exp, err = messageReceived.getValidRegularExpression(); err == nil {
			break
		}

		message.Text = ":scream: I don't understand that regular expression. Who does? Try again! \n" +
			"Type something like `It's /(?i)hello/` to match an answer like Hello, HELLO or hello world, " +
			"and don't forget write it between / and / but don't start with / \n" +
			"You may find some help in this website for help: https://regex101.com/"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}
	}

	var match bool
	for {
		message.Text = "Should I reply when the member's answer match the regular expression?"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

		messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
		if messageReceived.isCancel() {
			message.Text = ":ok_hand:"
			if err := message.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			return
		}

		if messageReceived.isYes() || messageReceived.isNo() {
			match = messageReceived.isYes()
			break
		}

		message.Text = ":scream: Type something like `yes`, `no` or `cancel`\n" +
			"If you type `no`, I will reply only if regular expression *doesn't match* the answer\n" +
			"If you type `yes`, only if regular expression *match* the answer\n"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

	}

	message.Text = "What do I should reply to the question?"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	messageReceived = <-channelsMap.p[m.getChannelID()]["<@"+m.User+">"]
	if messageReceived.isCancel() {
		message.Text = ":ok_hand:"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}

	reply := messageReceived.Text

	replyToAdd := api.PredefinedDailyReply{
		ChannelID: m.getChannelID(),
		Question:  question,
		Reply:     reply,
		Exp:       exp,
		Match:     match,
	}

	if err := addPredefinedReply(&replyToAdd); err != nil {
		log.Printf("slackutils: error storing predefined reply from channel %s: %s\n", m.getChannelID(), err)
		sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}

	message.Text = "Yeah! I will do it as you've requested :smiling_imp:"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending msj to channel %s: %s\n", m.getChannelID(), err)
	}
}

func manageDeleteReplyDaily(ws *websocket.Conn, m *Message) {

	channelsMap.Lock()
	if channelsMap.p[m.getChannelID()] == nil {
		channelsMap.p[m.getChannelID()] = map[string]chan Message{}
	}
	if channelsMap.p[m.getChannelID()]["<@"+m.User+">"] == nil {
		channelsMap.p[m.getChannelID()]["<@"+m.User+">"] = make(chan Message)
		defer channelsMap.finishWaitingMember(m.getChannelID(), "<@"+m.User+">")
	}
	channelsMap.Unlock()

	message := &Message{
		ID:      0,
		Type:    "message",
		User:    "",
		Channel: m.getChannelID(),
		Text:    "Who isn't going to participate more in the Daily Meeting?",
	}
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
	}

	// TODO: delete reply

	message.Text = "Predefined reply deleted"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error sending msj to channel %s: %s\n", m.getChannelID(), err)
	}
}

func isExpectedMessage(m *Message) bool {
	switch {
	case m.Type != "message":
		return false
	case m.getChannelID() == "":
		return false
	case channelsMap.isMemberAwaited(m.getChannelID(), m.User):
		return true
	default:
		return false
	}
}

func manageExpectedMessage(ws *websocket.Conn, m *Message) {
	channelsMap.Lock()
	channelsMap.p[m.getChannelID()]["<@"+m.User+">"] <- *m
	channelsMap.Unlock()
}

// Messages to send

func sendHelloMsj(ws *websocket.Conn, channelID string) error {

	m := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text: "Hello team! I'm here to help you with your daily meetings. To add members " +
			"to the daily meeting type `@leanmanager: daily add member`, to setup the hour of the " +
			"daily meeting, type `@leanmanager: daily schedule`.\n" +
			"If you need help, just type `@leanmanager: help` :sos:",
	}

	return m.send(ws)
}

func sendHelpMsj(ws *websocket.Conn, channelID string) error {

	m := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text: "Even if I'm a bit surly, work with me it's quite easy :sunglasses:\n" +
			"Just type the order, and I will obey. Those are the orders available:\n" +
			"`@leanmanager: daily add member` to add new members in the Daily Meeting\n" +
			"`@leanmanager: daily delete member` to delete members from the Daily Meeting\n" +
			"`@leanmanager: daily list members` to obtain a list of members participating in the Daily\n" +
			"`@leanmanager: daily start` to start the daily in any moment or to repeat it\n" +
			"`@leanmanager: daily info` to know when it's scheduled and the last time it was done\n" +
			"`@leanmanager: daily schedule` to setup the periodicity of the Daily Meeting\n" +
			"`@leanmanager: daily resume` to do the Daily report if you miss the Daily Meeting\n" +
			"`@leanmanager: daily add reply` to add predefined bot replies to the Daily answers\n" +
			"`@leanmanager: daily delete reply` to delete predefined bot replies to the Daily answers\n" +
			"If I ask something, just reply, I will do my best to understand you :grin:",
	}

	return m.send(ws)
}

func sendStartDailyMsj(ws *websocket.Conn, channelID string) error {
	m := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text:    "Hi @everyone! Let's start the Daily Meeting :mega:",
	}
	return m.send(ws)
}

func sendNotAvailableMsj(ws *websocket.Conn, channelID string) error {
	m := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text:    ":chicken:... please, do it later, just type `@leanmanager: daily resume` before the end of the day",
	}
	return m.send(ws)
}

func sendNotMembersRegisteredMsj(ws *websocket.Conn, channelID string) error {
	m := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text:    "There are no members registered yet. Type `@leanmanager: daily add member` to add the first one",
	}
	return m.send(ws)
}
func sendUnexpectedProblemMsj(ws *websocket.Conn, channelID string) error {
	m := &Message{
		ID:      0,
		Type:    "message",
		Channel: channelID,
		Text: "It was an unexpected behaviour, I don't have idea what's going to happen now... so you can " +
			"wait and see what happens or contact support@leanmanager.eu asking for help",
	}
	return m.send(ws)
}

// Message methods

func (m Message) send(ws *websocket.Conn) error {
	m.ID = counter.add(1)
	return websocket.JSON.Send(ws, m)
}

func (m Message) String() string {
	return fmt.Sprintf("Channel: %s, Type: %s, User: %s, ID: %d, Message: %s", m.Channel, m.Type, m.User, m.ID, m.Text)
}
func (m Message) getChannelID() string {
	if m.Type == "message" {
		return m.Channel.(string)
	}
	if m.Type == "group_joined" || m.Type == "channel_joined" {
		t := m.Channel.(map[string]interface{})
		for k, v := range t {
			if k == "id" {
				if str, ok := v.(string); ok {
					return str
				}
			}
		}
	}

	return ""
}

func (m Message) isHelpMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: help") ||
		strings.HasPrefix(m.Text, "leanmanager: help")) {
		return true
	}

	return false
}

func (m Message) isInitialMsj(botID string) bool {
	if m.Type == "group_joined" || m.Type == "channel_joined" {
		return true
	}

	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: hello") ||
		strings.HasPrefix(m.Text, "leanmanager: hello")) {
		return true
	}

	return false
}

func (m Message) isAddMemberDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily add member") ||
		strings.HasPrefix(m.Text, "leanmanager: daily add member")) {
		return true
	}

	return false
}

func (m Message) isDeleteMemberDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily delete member") ||
		strings.HasPrefix(m.Text, "leanmanager: daily delete member")) {
		return true
	}
	return false
}

func (m Message) isListMembersDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily list") ||
		strings.HasPrefix(m.Text, "leanmanager: daily list")) {
		return true
	}
	return false
}

func (m Message) isStartDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily start") ||
		strings.HasPrefix(m.Text, "leanmanager: daily start")) {
		return true
	}
	return false
}

func (m Message) isInfoDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily info") ||
		strings.HasPrefix(m.Text, "leanmanager: daily info")) {
		return true
	}
	return false
}

func (m Message) isAddReplyDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily add reply") ||
		strings.HasPrefix(m.Text, "leanmanager: daily add reply")) {
		return true
	}
	return false
}

func (m Message) isDeleteReplyDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily delete reply") ||
		strings.HasPrefix(m.Text, "leanmanager: daily delete reply")) {
		return true
	}
	return false
}

func (m Message) isScheduleDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily schedule") ||
		strings.HasPrefix(m.Text, "leanmanager: daily schedule")) {
		return true
	}
	return false
}

func (m Message) isResumeDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: daily resume") ||
		strings.HasPrefix(m.Text, "leanmanager: daily resume")) {
		return true
	}
	return false
}

func (m Message) isCommand(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: ") ||
		strings.HasPrefix(m.Text, "leanmanager: ")) {
		return true
	}
	return false
}

func (m Message) isYes() bool {
	if m.Type != "message" {
		return false
	}
	return strings.EqualFold(m.Text, "si") || strings.EqualFold(m.Text, "yes") ||
		strings.EqualFold(m.Text, "sip") || strings.EqualFold(m.Text, "sí") ||
		strings.EqualFold(m.Text, "yeah") || strings.EqualFold(m.Text, "ok")
}

func (m Message) isNo() bool {
	if m.Type != "message" {
		return false
	}
	return strings.EqualFold(m.Text, "no") || strings.EqualFold(m.Text, "nop")
}

func (m Message) isCancel() bool {
	if m.Type != "message" {
		return false
	}
	return strings.EqualFold(m.Text, "cancel")
}

func (m Message) getValidDays() (doW []time.Weekday) {
	if m.Type != "message" {
		return nil
	}

	re := regexp.MustCompile("(?i)(every|week|mon|tues|wednes|thurs|fri|satur|sun)[d][a][y]s?")
	days := re.FindAllString(m.Text, 7)

	for _, d := range days {
		switch strings.ToLower(d) {
		case "monday":
			doW = append(doW, time.Monday)
		case "tuesday":
			doW = append(doW, time.Tuesday)
		case "wednesday":
			doW = append(doW, time.Wednesday)
		case "thursday":
			doW = append(doW, time.Thursday)
		case "friday":
			doW = append(doW, time.Friday)
		case "saturday":
			doW = append(doW, time.Saturday)
		case "sunday":
			doW = append(doW, time.Sunday)
		case "weekday", "weekdays":
			doW = append(doW, []time.Weekday{time.Monday, time.Tuesday, time.Wednesday,
				time.Thursday, time.Friday}...)
		case "everyday":
			doW = append(doW, []time.Weekday{time.Monday, time.Tuesday, time.Wednesday,
				time.Thursday, time.Friday, time.Saturday, time.Sunday}...)
		}
	}

	return doW
}

func (m Message) getValidHour() string {
	if m.Type != "message" {
		return ""
	}

	re := regexp.MustCompile("(?i)[0-2]?[0-9]:[0-9][0-9][A|P]?M?")
	return re.FindString(m.Text)
}

func (m Message) getValidUserIDs() []string {
	if m.Type != "message" {
		return nil
	}

	re := regexp.MustCompile("(?i)[<][@][A-Za-z0-9]*[>]")
	return re.FindAllString(m.Text, -1)
}

func (m Message) getPredefinedReply(q int) string {
	if m.Type != "message" {
		return ""
	}

	replies, err := listPredefinedReplies(m.getChannelID())
	if err != nil {
		log.Printf("slackutils: error accessing predefined replies: %v\n", err)
		return ""
	}

	for i := 0; i < len(*replies); i++ {

		if (*replies)[i].ChannelID != m.getChannelID() || (*replies)[i].Question != q {
			continue
		}

		re, err := regexp.Compile((*replies)[i].Exp)
		if err != nil {
			log.Printf("slackutils: there is a wrong regex in the predefined replies: %v\n", err)
			continue
		}
		exp := re.FindString(m.Text)
		if exp == "" && (*replies)[i].Match == true {
			return (*replies)[i].Reply
		}
		if exp != "" && (*replies)[i].Match == false {
			return (*replies)[i].Reply
		}
	}

	return ""
}

func (m Message) getValidAnswer() (int, error) {
	if m.Type != "message" {
		return -1, fmt.Errorf("no type message")
	}

	re := regexp.MustCompile("(?i)([f][i][r][s][t]|[s][e][c][o][n][d]|[l][a][s][t])")
	switch strings.ToLower(re.FindString(m.Text)) {
	case "first":
		return 0, nil
	case "second":
		return 1, nil
	case "last":
		return 2, nil
	}
	return -1, fmt.Errorf("question not found")
}

func (m Message) getValidRegularExpression() (string, error) {
	if m.Type != "message" {
		return "", fmt.Errorf("no type message")
	}

	re := regexp.MustCompile("([/].*[/])")
	exp := re.FindString(m.Text)

	if exp == "" {
		return "", fmt.Errorf("no regular expression ")
	}

	_, err := regexp.Compile(exp)

	if len(exp) > 2 {
		return exp[1 : len(exp)-1], err
	}
	return "", err
}

// Message methods

func (pe *pendingMsjController) isMemberAwaited(channelID, memberID string) bool {
	pe.Lock()
	defer pe.Unlock()
	if pe.p[channelID] == nil {
		return false
	}

	if pe.p[channelID]["<@"+memberID+">"] != nil {
		return true
	}
	return false

}

func (pe *pendingMsjController) finishWaitingMember(channelID, memberID string) {
	pe.Lock()
	close(channelsMap.p[channelID][memberID])
	delete(channelsMap.p[channelID], memberID)
	pe.Unlock()
}

func (ac *atomicCounter) add(i uint64) uint64 {
	ac.Lock()
	ac.i += i
	ac.Unlock()
	return ac.i
}
