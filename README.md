# leanmanager.eu
Do you know how much your PR costs?

## Introduction

This should be an end-to-end solution for development management using Github, Slack and mail. The mantra is "Keep it simple", we want to focus in development, not in management tools, time tracking and so on.

## Build and execute

```
go get github.com/boltdb/bolt
go get golang.org/x/net/websocket
./leanmanager
```

TODO: see how to automate it / vendoring?

## Slack bot

Our main point of contact with leanmanager, use slack https://api.slack.com/rtm. 

We want to have the following features:

### Daily meetings

- [x] Launch the bot periodically and print a message
- [x] Register the channel and all the members
- [x] Make the questions to all members
- [ ] Store the response of each member
- [ ] Package it as an Slack App (ready to deal with OAuth?)
- [ ] Some improvements to the bot (icon, etc.)
- [ ] Put in docker and automatic deployment (Travis?)
- [ ] Better login, identify the admin

```
	/*
	// TODO: {0 group_left G1E3T1U1W }
	*/
```

### Ask for reports 

- [ ] Define scope
- [ ] Send by email

### Make exceptions 

- [ ] Reminders: team member X is on holidays, fill your hours, etc.

## Github bot

We want to know how much time has been spent for an specific PR.

### Github bot TODO

- [ ] Define scope

## Calendar bot

Users must be able to fill the time using GCalendar / Outlook

### Calendar bot TODO

- [ ] Define scope

## Trello board

Have a board where we can prioritized our issues, see PR associated and so on.

### Trello board TODO

- [ ] Define scope
