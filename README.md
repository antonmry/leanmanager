# leanmanager
It's time to replace your manager with a bot!

## Introduction

The aim of Lean Manager is to be an end-to-end solution for management of development teams using your favourite tools, not adding more. The mantra is "Keep it simple" so we can focus in development and product design, not in management, time tracking and so on.

## Slack bot

It's our main point of contact with leanmanager, it uses slack [Real Time API](https://api.slack.com/rtm). Rigth now the only functionality is running the Daily meetings making the questions but more functionality will be added.

To install leanmanager, you need to install previously go, my recomendation is to use the [official binary ditributions](https://golang.org/doc/install).

Then, just execute:

```sh
go get -v github.com/antonmry/leanmanager
```

### Daily meetings

Daily meetings are in phase beta, but you can use them as you can see in the following screenshot:

![Daily screenshot with leanmanager](resources/daily.png)


To run it, you need to create a bot in the [slack bot creation page](https://my.slack.com/services/new/bot) and retrieve the token of your new bot. Then execute:

```sh
leanmanager apiserver 
leanmanager slackbot -t YOUR_TOKEN
```

By default, leanmanager stores the database in /tmp. If you want to persist it, you can execute:

```sh
leanmanager apiserver -d /YOUR/PATH -n NAME_OF_THE_DB
```
