package main

import (
	"fmt"
	"strings"
	"time"
	"golang.org/x/net/websocket"
)

const (
	slackToken = "xoxb-47966556151-mh3PMjIpzo1vUYImrqFAfCw2"
)


func main() {
	// websocket based!
	ws, id := slackConnect(slackToken)
	fmt.Println("Slack: bot connected")

	// Keepalives
	t := time.NewTicker(5 * time.Second)
	go func() {
		for {
			periodicFunction(ws)
			<-t.C
		}
	}()

	for {
		// read messages
		m, err := receiveMessage(ws)
		if err != nil {
			fmt.Println("Slack: error receiving message")
			continue
		}

		if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">") {
			parts := strings.Fields(m.Text)
			if strings.Contains(strings.Join(parts, ""), "hello") {
				go func(m Message) {
					m.Text = "Hello world"
					sendMessage(ws, m)
				}(m)
			} else {
				m.Text = fmt.Sprintf("Message not understood\n")
				sendMessage(ws, m)
			}
		}
	}
}

func periodicFunction(ws *websocket.Conn) {

	// TODO: this is failing, why?
	// https://api.slack.com/rtm

	fmt.Println("Periodic call invoked")
	m := &Message{0, "message", "daily", "Periodic message" }
	err := sendMessage(ws, *m)

	if err != nil {
		fmt.Printf("Slack: error sending message: %s\n", err)
	}
}
