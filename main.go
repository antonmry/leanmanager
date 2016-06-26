// Cobra package to launch the commands defined in the cmd package
package main

import (
	"fmt"
	"github.com/antonmry/leanmanager/cmd"
	"os"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
