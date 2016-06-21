# TODO

## Technical debt

- [ ] create the API Server as k8s does, it will be the heart of the project #1
- [ ] Put in docker and automatic deployment (Travis?)
- [ ] avoid sync/atomic, check sync.WaitGroup https://blog.golang.org/pipelines
- [ ] create issues for these items
- [ ] move database access from global variable to interface
- [ ] unit testing!!
- [ ] do we need vendoring?
- [x] restructure the project
- [x] const must be read as ENV variable or args, create BASH script to launch
- [x] move open DB to dbutils #1
- [ ] refactor manageMessage, it's too big
 
## Slackbot

### Daily meetings

- [ ] how we differ messages by channel and user? channelID shouldn't be a global variable #2
- [ ] timeouts: https://gobyexample.com/timeouts #2
- [ ] avoid @leanmanager prefix when possible! #2
- [ ] if error receiving messange, we should reconnect!! #2
- [ ] check if newMember is member of the channel when added #2
- [ ] allow to add many members with one command #2
- [ ] schedule the daily meeting #2
- [ ] show help commands #2 
- [ ] Package it as an Slack App (ready to deal with OAuth?)
- [ ] Some improvements to the bot (icon, etc.)
- [ ] Store the response of each member and do what?
- [ ] Better login, identify the admin
- [x] Launch the bot periodically and print a message
- [x] Register the channel and all the members
- [x] Make the questions to all members

### Ask for reports 

- [ ] Define scope
- [ ] Send by email

## Make exceptions 

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
