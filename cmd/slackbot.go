// Package cmd implements the leanmanager available commands
package cmd

import (
	"log"
	"os"

	"github.com/antonmry/leanmanager/slackbot"
	"github.com/spf13/cobra"
)

var slackbotCmd = &cobra.Command{
	Use:   "slackbot",
	Short: "Launch the bot and connect to Slack",
	Long: `It will run the slackbot to receive and send to slack messages. It will save the data in the
	path provided or in /tmp if not provided.`,
	Run: func(cmd *cobra.Command, args []string) {
		if slackToken == "" {
			slackToken = os.Getenv("LEANMANAGER_TOKEN")
		}

		if slackToken == "" {
			log.SetFlags(0)
			log.Fatal("Please, specify slackToken using -t or --slackToken")
		}
		slackbot.LaunchSlackbot(slackToken, teamName, apiserverHost, apiserverPort)
	},
}

func init() {
	RootCmd.AddCommand(slackbotCmd)
}
