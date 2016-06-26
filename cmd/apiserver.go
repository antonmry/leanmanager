// Package cmd implements the leanmanager available commands
package cmd

import (
	"github.com/antonmry/leanmanager/apiserver"
	"github.com/spf13/cobra"
)

// apiserverCmd represents the apiserver command
var apiserverCmd = &cobra.Command{
	Use:   "apiserver",
	Short: "The APIs which control the leanmanager's behaviour",
	Long:  `.`,
	Run: func(cmd *cobra.Command, args []string) {
		apiserver.LaunchAPIServer(pathDb, dbName, apiserverHost, apiserverPort)
	},
}

func init() {
	RootCmd.AddCommand(apiserverCmd)
}
