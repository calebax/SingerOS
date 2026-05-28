package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/ygpkg/yg-go/lifecycle"
	"github.com/ygpkg/yg-go/logs"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/internal/cli"
)

var (
	sessionServerAddr  string
	sessionJSON        bool
	sessionKeyword     string
	sessionStatus      string
	sessionType        string
	sessionAssistantID uint
	sessionOffset      int
	sessionLimit       int
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage sessions",
	Long:  `Manage sessions in the Leros platform.`,
}

var sessionLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List sessions",
	Long:  `List all sessions with optional filtering.`,
	Run: func(cmd *cobra.Command, args []string) {
		go func() {
			req := &contract.ListSessionsRequest{
				Pagination: contract.ListSessionsRequest{}.Pagination,
			}
			req.Offset = sessionOffset
			req.Limit = sessionLimit
			req.Fill()

			if sessionKeyword != "" {
				req.Keyword = &sessionKeyword
			}
			if sessionStatus != "" {
				req.Status = &sessionStatus
			}
			if sessionType != "" {
				req.Type = &sessionType
			}
			if cmd.Flags().Changed("assistant-id") {
				req.AssistantID = &sessionAssistantID
			}

			result, err := cli.ListSessions(lifecycle.Std().Context(), sessionServerAddr, req)
			if err != nil {
				logs.Errorf("list sessions: %v", err)
				lifecycle.Std().Exit()
				return
			}
			printSessions(result)
			lifecycle.Std().Exit()
		}()
		lifecycle.Std().WaitExit()
	},
}

func printSessions(list *contract.SessionList) {
	if sessionJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(list.Items)
		return
	}

	if len(list.Items) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PUBLIC_ID\tTYPE\tSTATUS\tTITLE\tMSG_COUNT\tCREATED_AT")
	for _, s := range list.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
			s.SessionID, s.Type, s.Status, s.Title, s.MessageCount,
			s.CreatedAt.Format("2006-01-02T15:04:05Z"))
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "\nTotal: %d, Offset: %d, Limit: %d\n", list.Total, list.Offset, list.Limit)
}

func init() {
	sessionLsCmd.Flags().StringVar(&sessionServerAddr, "server-addr", "127.0.0.1:8080", "Leros server address (host:port)")
	sessionLsCmd.Flags().BoolVar(&sessionJSON, "json", false, "Output in JSON format")
	sessionLsCmd.Flags().StringVar(&sessionKeyword, "keyword", "", "Filter by title or public_id keyword")
	sessionLsCmd.Flags().StringVar(&sessionStatus, "status", "", "Filter by status")
	sessionLsCmd.Flags().StringVar(&sessionType, "type", "", "Filter by session type")
	sessionLsCmd.Flags().UintVar(&sessionAssistantID, "assistant-id", 0, "Filter by assistant ID")
	sessionLsCmd.Flags().IntVar(&sessionOffset, "offset", 0, "Pagination offset")
	sessionLsCmd.Flags().IntVar(&sessionLimit, "limit", 20, "Pagination limit")

	sessionCmd.AddCommand(sessionLsCmd)
	rootCmd.AddCommand(sessionCmd)
}
