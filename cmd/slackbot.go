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

package cmd

import (
	"github.com/antonmry/leanmanager/slackbot"
	"github.com/spf13/cobra"
	"log"
)

var (
	slackToken  string
	pathLocalDb string
	teamName    string
)

var slackbotCmd = &cobra.Command{
	Use:   "slackbot",
	Short: "Launch the bot and connect to Slack",
	Long: `It will run the slackbot to receive and send to slack messages. It will save the data in the
	path provided or in /tmp if not provided.`,
	Run: func(cmd *cobra.Command, args []string) {
		if slackToken == "" {
			log.SetFlags(0)
			log.Fatal("Please, specify slackToken using -t or --slackToken")
		}
		slackbot.LaunchSlackbot(slackToken, pathLocalDb, teamName)
	},
}

func init() {
	RootCmd.AddCommand(slackbotCmd)

	slackbotCmd.PersistentFlags().StringVarP(&slackToken, "slackToken", "t", "", "Token used to connect to Slack.")
	slackbotCmd.PersistentFlags().StringVarP(&pathLocalDb, "pathLocalDb", "l", "/tmp",
		"The path to store the slackbot Db")
	slackbotCmd.PersistentFlags().StringVarP(&teamName, "teamName", "n", "YOURTEAMNAME", "Name of the bot's team.")
}
