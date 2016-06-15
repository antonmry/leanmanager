package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"golang.org/x/net/websocket"
)

var counter uint64

type Message struct {
	Id      uint64 `json:"id"`
	Type    string `json:"type"`
	Channel interface{} `json:"channel"`
	Text    string `json:"text"`
}

type Channel struct {
	Id		string `json:"id"`
	Name		string `json:"name"`
	IsChannel	string `json:"is_channel"`
	Members         interface{} `json:"members"`
}

type Member string

type responseRtmStart struct {
	Ok    bool         `json:"ok"`
	Error string       `json:"error"`
	Url   string       `json:"url"`
	Self  responseSelf `json:"self"`
}

type responseSelf struct {
	Id string `json:"id"`
}

func slackConnect(token string) (*websocket.Conn, string) {
	wsurl, id, err := slackInit(token)
	if err != nil {
		fmt.Printf("Slack: error in init: %s", err)
		return nil, ""
	}

	ws, err := websocket.Dial(wsurl, "", "https://api.slack.com/")
	if err != nil {
		fmt.Printf("Slack: error creating websocket: %s", err)
		return nil, ""
	}

	return ws, id
}

func slackInit(token string) (wsurl, id string, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("Slack: error Get: %s", err)
	}
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("Slack: error response from Get: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", "", fmt.Errorf("Slack: error reading Body: %s", err)
	}

	var slackResp responseRtmStart
	err = json.Unmarshal(body, &slackResp)
	if err != nil {
		return "", "", fmt.Errorf("Slack: error parsing Slack resp: %s", err)
	}

	if !slackResp.Ok {
		return "", "", fmt.Errorf("Slack: error Slack: %s", err)
	}

	wsurl = slackResp.Url
	id = slackResp.Self.Id
	return
}


func receiveMessage(ws *websocket.Conn) (m Message, err error) {
	err = websocket.JSON.Receive(ws, &m)
	return
}

func sendMessage(ws *websocket.Conn, m Message) error {
	m.Id = atomic.AddUint64(&counter, 1)
	return websocket.JSON.Send(ws, m)
}

