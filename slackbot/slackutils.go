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
	"encoding/json"
	"fmt"
	"golang.org/x/net/websocket"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

var counter uint64

type Message struct {
	Id      uint64      `json:"id"`
	Type    string      `json:"type"`
	Channel interface{} `json:"channel"`
	Text    string      `json:"text"`
}

type Channel struct {
	Id        string      `json:"id"`
	Name      string      `json:"name"`
	IsChannel string      `json:"is_channel"`
	Members   interface{} `json:"members"`
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

func slackConnect(token string) (ws *websocket.Conn, id string, err error) {
	wsurl, id, err := slackInit(token)
	if err != nil {
		return nil, "", fmt.Errorf("slackutils: error initiating communication with slack: %s", err)
	}

	ws, err = websocket.Dial(wsurl, "", "https://api.slack.com/")
	if err != nil {
		return nil, "", fmt.Errorf("slackutils: error creating websocket: %s", err)
	}

	return ws, id, nil
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
