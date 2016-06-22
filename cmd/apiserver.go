// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
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
	"github.com/antonmry/leanmanager/apiserver"
	"github.com/spf13/cobra"
)

var (
	pathDb string
	dbName string
	host   string
	port   int
)

// apiserverCmd represents the apiserver command
var apiserverCmd = &cobra.Command{
	Use:   "apiserver",
	Short: "The APIs which control the leanmanager's behaviour",
	Long:  `.`,
	Run: func(cmd *cobra.Command, args []string) {
		apiserver.LaunchAPIServer(pathDb, dbName, host, port)
	},
}

func init() {
	RootCmd.AddCommand(apiserverCmd)

	apiserverCmd.PersistentFlags().StringVarP(&pathDb, "pathdb", "d", "/tmp", "The path to store the slackbot Db")
	apiserverCmd.PersistentFlags().StringVarP(&dbName, "dbname", "n", "leanmanager",
		"Name of the DB where data is stored")
	slackbotCmd.PersistentFlags().StringVarP(&host, "host", "o", "localhost", "IP or hostname of your leanmanager API server.")
	slackbotCmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "IP or hostname of your leanmanager API server.")
}
