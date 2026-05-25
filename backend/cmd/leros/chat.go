package main

import (
	"github.com/spf13/cobra"
	"github.com/ygpkg/yg-go/lifecycle"
	"github.com/ygpkg/yg-go/logs"

	"github.com/insmtx/Leros/backend/internal/cli"
)

var chatServerAddr string

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Start an interactive chat session with Leros",
	Long: `Start an interactive chat session with a running Leros server.
Streams assistant responses in real-time via SSE.

If a message is provided as a positional argument, it will be sent as the
first message. After that, you can continue typing messages interactively.
Type /exit or /quit to end the session.

The server must be running. By default, connects to 127.0.0.1:8080.
Set LEROS_DEV=true if the server is running in development mode.`,
	Run: func(cmd *cobra.Command, args []string) {
		var initialMessage string
		if len(args) > 0 {
			initialMessage = args[0]
		}

		go func() {
			if err := cli.Chat(lifecycle.Std().Context(), chatServerAddr, initialMessage); err != nil {
				logs.Errorf("chat: %v", err)
			}
			lifecycle.Std().Exit()
		}()
		lifecycle.Std().WaitExit()
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatServerAddr, "server-addr", "127.0.0.1:8080", "Leros server address (host:port)")
	rootCmd.AddCommand(chatCmd)
}
