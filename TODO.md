# TODO

## Technical debt

- [ ] Don't panic if API Server is down
- [ ] Avoid unexpected message if channel isn't defined
- [ ] unit testing!!
- [ ] Automatic deployment (Travis?)
- [ ] do we need Godep?
- [ ] [Avoid Go's default client](https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779#.tmgmfnr34)
- [ ] Move daily meeting logic to the API Server 
- [ ] move database access from global variable to interface
- [ ] if error receiving messange, we should reconnect!! 
- [ ] Fix member.Name vs. member.ID
- [x] Put in docker #7
- [x] avoid sync/atomic
- [x] refactor manageMessage, it's too big
- [x] @leanmanager: add without panics
- [x] restructure the project
- [x] const must be read as ENV variable or args, create BASH script to launch
- [x] create the API Server as k8s does, it will be the heart of the project #1
- [x] move open DB to dbutils #1

## Slackbot

### Daily meetings

- [ ] check availabilty of members before launch the Daily
- [ ] skip the daily by holidays
- [ ] add all members of the channel
- [ ] Package it as an Slack App (ready to deal with OAuth?)
- [ ] Some improvements to the bot (icon, etc.)
- [ ] Store the response of each member and do what?
- [ ] check if newMember is member of the channel when added 
- [ ] Add timezones to the bot
- [ ] Limit time range for the daily to 12 hours
- [ ] Add a "Good morning" feature
- [ ] Better login, identify the admin
- [ ] Scrapper daily jokes from reddit, dilbert and so on ;-)
- [x] validate responses (contain a Github PR or a Github Issue) #3
- [x] show help commands #3
- [x] schedule the daily meeting #3
- [x] resume command #3
- [x] timeouts: https://gobyexample.com/timeouts #3
- [x] avoid @leanmanager prefix when possible! #2
- [x] differ messages by channel and user, channelID shouldn't be a global variable #2
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
