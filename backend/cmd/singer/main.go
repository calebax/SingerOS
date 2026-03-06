package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/ygpkg/yg-go/logs"
)

var rootCmd = &cobra.Command{
	Use:   "singer",
	Short: "Backend service for the SingerOS Backend",
	Long:  `This is the backend service for the SingerOS Backend, responsible for handling API requests and business logic.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Start the backend service
		logs.Info("Starting backend service...")
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logs.Errorf("Error executing command: %v", err)
		os.Exit(1)
	}
}
