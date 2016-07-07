// Package api contains the types used and exposed by the API Server
package api

import "time"

// Member represents a member of the team
type Member struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ChannelID string `json:"channelId"`
	TeamID    string `json:"teamId"`
}

// Channel represents a channel or group where members chat
type Channel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	TeamID string `json:"teamId"`
}

// Team represents a group of persons who work together
type Team struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"slackToken"`
}

// DailyMeeting represents a Daily Meeting with its status
type DailyMeeting struct {
	ChannelID string         `json:"channelId"`
	LastDaily time.Time      `json:"lastDaily"`
	StartTime time.Time      `json:"startTime"`
	LimitTime time.Time      `json:"limitTime"`
	Days      []time.Weekday `json:"days"`
}

// PredefinedDailyReply represents an automated reply to answers in the Daily Meeting following the exp criteria
type PredefinedDailyReply struct {
	ChannelID string `json:"channelId"`
	Question  int    `json:"question"`
	Reply     string `json:"reply"`
	Exp       string `json:"regularExpression"`
}
