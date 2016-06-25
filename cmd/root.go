// Package cmd implements the leanmanager available commands
package cmd

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/antonmry/leanmanager/apiserver"
	"github.com/antonmry/leanmanager/slackbot"
	"github.com/spf13/cobra"
)

var (
	slackToken    string
	teamName      string
	apiserverHost string
	apiserverPort int
	pathDb        string
	dbName        string
)

// RootCmd acts as an standalone instance launching all services to provide non-HA functionality
var RootCmd = &cobra.Command{
	Use:   "leanmanager",
	Short: "Replace your managers with a bot",
	Long: `This bot automates the tasks usually done by managers in development teams, so you can save costs and
	let your team work in more productive tasks than simple management.`,
	Run: func(cmd *cobra.Command, args []string) {
		if slackToken == "" {
			slackToken = os.Getenv("LEANMANAGER_TOKEN")
		}

		if slackToken == "" {
			log.SetFlags(0)
			log.Fatal("Please, specify slackToken using -t or --slackToken")
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			slackbot.LaunchSlackbot(slackToken, teamName, apiserverHost, apiserverPort)
		}()
		go func() {
			defer wg.Done()
			apiserver.LaunchAPIServer(pathDb, dbName, apiserverHost, apiserverPort)
		}()
		wg.Wait()
	},
}

// Execute is used by the root main to launch leanmanager commands
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	f := RootCmd.PersistentFlags()

	f.StringVarP(&pathDb, "pathdb", "d", "/tmp", "The path to store the slackbot Db")
	f.StringVarP(&dbName, "dbname", "n", "leanmanager", "Name of the DB where data is stored")
	f.StringVarP(&slackToken, "slackToken", "t", "", "Token used to connect to Slack.")
	f.StringVarP(&teamName, "teamName", "e", "YOURTEAMNAME", "Name of the bot's team.")
	f.StringVarP(&apiserverHost, "apiserverHost", "a", "localhost", "IP or hostname of your leanmanager API server.")
	f.IntVarP(&apiserverPort, "apiserverPort", "p", 8080, "IP or hostname of your leanmanager API server.")
}
