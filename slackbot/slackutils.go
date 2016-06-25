// Package slackbot provides all the leanmanager logic for the Slack bot
package slackbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/antonmry/leanmanager/api"

	"golang.org/x/net/websocket"
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

type pendingMsjController struct {
	sync.Mutex
	p map[string]map[string]chan Message
}

var counter = atomicCounter{}

var channelsMap = pendingMsjController{
	p: make(map[string]map[string]chan Message),
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
		Id:     m.getChannelID(),
		Name:   m.getChannelID(),
		TeamId: teamID}

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

func manageAddMember(ws *websocket.Conn, m *Message) {
	message := &Message{
		ID:      0,
		Type:    "message",
		Channel: m.getChannelID(),
		Text:    "",
	}
	if m.Text[strings.Index(m.Text, ": add")+5:] == "" {
		message.Text = ":godmode: Good try, but I don't panic so easily. Team member can't be empty"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending msj to channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}

	newMember := api.Member{
		Id:        m.Text[strings.Index(m.Text, ": add")+6:],
		Name:      m.Text[strings.Index(m.Text, ": add")+6:],
		ChannelId: m.getChannelID(),
		TeamId:    teamID,
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

func manageDelMember(ws *websocket.Conn, m *Message) {
	message := &Message{
		ID:      0,
		Type:    "message",
		Channel: m.getChannelID(),
		Text:    "",
	}

	if m.Text[strings.Index(m.Text, ": delete")+8:] == "" {
		message.Text = ":godmode: Good try, but I don't panic so easily. Team member can't be empty"
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending msj to channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}
	memberToBeDeleted := api.Member{
		Id:        m.Text[strings.Index(m.Text, ": delete")+9:],
		Name:      m.Text[strings.Index(m.Text, ": delete")+9:],
		ChannelId: m.getChannelID(),
		TeamId:    teamID,
	}

	if err := delTeamMember(&memberToBeDeleted); err != nil {
		log.Printf("slackutils: error invoking API Server to delete member %s in channel %s: %s",
			memberToBeDeleted.Id, m.getChannelID(), err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}
	message.Text = "Team member " + memberToBeDeleted.Name + " deleted"
	if err := message.send(ws); err != nil {
		log.Printf("slackutils: error deleting member in channel %s: %s\n", m.getChannelID(), err)
	}
	return
}

func manageListMembers(ws *websocket.Conn, m *Message) {

	teamMembers, err := listTeamMembers(m.getChannelID())

	if err != nil {
		log.Printf("slackutils: error invoking API Server to retrieve members of channel: %v", err)
		_ = sendUnexpectedProblemMsj(ws, m.getChannelID())
		return
	}

	if len(teamMembers[:]) == 0 {
		message := &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text: "There are no members registered yet. Type " +
				"`@leanmanager: add @username` to add the first one",
		}
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error listing member in channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}

	var b bytes.Buffer
	b.WriteString("Members registered for the next Daily Sprint: ")

	for i := 0; i < len(teamMembers[:]); i++ {
		b.WriteString(teamMembers[i].Name + ", ")
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
		message := &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text: "There are no members registered yet. Type " +
				"`@leanmanager: add @username` to add the first one",
		}
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error listing member in channel %s: %s\n", m.getChannelID(), err)
		}
		return
	}

	var messageReceived Message

	for i := 0; i < len(teamMembers[:]); i++ {

		message := &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text:    "Hi " + teamMembers[i].Id + "! Are you ready?.",
		}
		if err := message.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

		channelsMap.Lock()
		if channelsMap.p[m.getChannelID()] == nil {
			channelsMap.p[m.getChannelID()] = map[string]chan Message{}
		}
		channelsMap.p[m.getChannelID()][teamMembers[i].Id] = make(chan Message)
		channelsMap.Unlock()
		defer channelsMap.finishWaitingMember(m.getChannelID(), teamMembers[i].Id)

		for {
			messageReceived = <-channelsMap.p[m.getChannelID()][teamMembers[i].Id]
			if messageReceived.isYes() || messageReceived.isNo() {
				break
			}
		}

		if messageReceived.isNo() {
			dailyMeetingMessage := &Message{
				ID:      0,
				Type:    "message",
				Channel: m.getChannelID(),
				Text:    "Ok, you can do it later, just type `@leanmanager resume` before the end of the day",
			}
			if err := dailyMeetingMessage.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
			continue
		}

		if messageReceived.isYes() {
			dailyMeetingMessage := &Message{
				ID:      0,
				Type:    "message",
				Channel: m.getChannelID(),
				Text:    teamMembers[i].Name + ", what did you do yesterday?.",
			}
			if err := dailyMeetingMessage.send(ws); err != nil {
				log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
			}
		}

		messageReceived = <-channelsMap.p[m.getChannelID()][teamMembers[i].Id]
		dailyMeetingMessage := &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text:    teamMembers[i].Name + ", what will you do today?.",
		}
		if err := dailyMeetingMessage.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

		messageReceived = <-channelsMap.p[m.getChannelID()][teamMembers[i].Id]
		dailyMeetingMessage = &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text:    teamMembers[i].Name + ", are there any impediments in your way?.`",
		}
		if err := dailyMeetingMessage.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}

		messageReceived = <-channelsMap.p[m.getChannelID()][teamMembers[i].Id]
		dailyMeetingMessage = &Message{
			ID:      0,
			Type:    "message",
			Channel: m.getChannelID(),
			Text:    "Thanks " + teamMembers[i].Name,
		}
		if err := dailyMeetingMessage.send(ws); err != nil {
			log.Printf("slackutils: error sending message to channel %s: %s\n", m.getChannelID(), err)
		}
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
			"to the daily meeting type `@leanmanager: add @username`, to setup the hour of the " +
			"daily meeting, type something like `@leanmanager: schedule monday tuesday friday 13:00`.\n" +
			"If you need help, just type `@leanmanager: help`",
	}

	m.ID = counter.add(1)
	return websocket.JSON.Send(ws, m)
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

func (m Message) isInitialMsj(botID string) bool {
	if m.Type == "group_joined" || m.Type == "channel_joined" {
		return true
	}

	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: hello") ||
		strings.HasPrefix(m.Text, botID+": hello")) {
		return true
	}

	return false
}

func (m Message) isAddMemberDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: add") ||
		strings.HasPrefix(m.Text, botID+": add")) {
		return true
	}

	return false
}

func (m Message) isDeleteMemberDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: delete") ||
		strings.HasPrefix(m.Text, botID+": delete")) {
		return true
	}
	return false
}

func (m Message) isListMemersDailyMsj(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: list") ||
		strings.HasPrefix(m.Text, botID+": list")) {
		return true
	}
	return false
}

func (m Message) isStartDaily(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: start") ||
		strings.HasPrefix(m.Text, botID+": start")) {
		return true
	}
	return false
}

func (m Message) isCommand(botID string) bool {
	if m.Type == "message" && (strings.HasPrefix(m.Text, "<@"+botID+">: ") ||
		strings.HasPrefix(m.Text, botID+": ")) {
		return true
	}
	return false
}

func (m Message) isYes() bool {
	if m.Type != "message" {
		return false
	}
	if len(m.Text) > 2 {
		switch m.Text[len(m.Text)-3 : len(m.Text)] {
		case "yes", "Yes", "YES", "yep", "si", "Sip", "sip", "sí", "Sí":
			return true
		}
	}
	return false

}

func (m Message) isNo() bool {
	if m.Type != "message" {
		return false
	}
	if len(m.Text) > 1 {
		switch m.Text[len(m.Text)-2 : len(m.Text)] {
		case "No", "NO", "no":
			return true
		}
	}
	if len(m.Text) > 2 {
		switch m.Text[len(m.Text)-3 : len(m.Text)] {
		case "Nop", "NOP", "nop":
			return true
		}
	}
	return false
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
	defer pe.Unlock()
	close(channelsMap.p[channelID][memberID])
	delete(channelsMap.p[channelID], memberID)
}

func (ac *atomicCounter) add(i uint64) uint64 {
	ac.Lock()
	defer ac.Unlock()
	ac.i += i
	return ac.i
}
