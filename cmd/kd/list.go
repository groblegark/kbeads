package main

import (
	"context"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List beads",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetStringSlice("status")
		beadType, _ := cmd.Flags().GetStringSlice("type")
		kind, _ := cmd.Flags().GetStringSlice("kind")
		limit, _ := cmd.Flags().GetInt("limit")
		assignee, _ := cmd.Flags().GetString("assignee")
		offset, _ := cmd.Flags().GetInt("offset")

		req := &client.ListBeadsRequest{
			Status:   status,
			Type:     beadType,
			Kind:     kind,
			Limit:    limit,
			Assignee: assignee,
			Offset:   offset,
		}

		resp, err := beadsClient.ListBeads(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadListJSON(resp.Beads)
		} else {
			printBeadListTable(resp.Beads, resp.Total)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringSliceP("status", "s", nil, "filter by status (repeatable)")
	listCmd.Flags().StringSliceP("type", "t", nil, "filter by type (repeatable)")
	listCmd.Flags().StringSliceP("kind", "k", nil, "filter by kind (repeatable)")
	listCmd.Flags().Int("limit", 20, "maximum number of beads to return")
	listCmd.Flags().String("assignee", "", "filter by assignee")
	listCmd.Flags().Int("offset", 0, "offset for pagination")
}
