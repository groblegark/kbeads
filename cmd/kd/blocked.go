package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Show blocked beads",
	RunE: func(cmd *cobra.Command, args []string) error {
		assignee, _ := cmd.Flags().GetString("assignee")
		beadType, _ := cmd.Flags().GetStringSlice("type")
		limit, _ := cmd.Flags().GetInt32("limit")

		req := &beadsv1.ListBeadsRequest{
			Status:   []string{"blocked"},
			Type:     beadType,
			Assignee: assignee,
			Limit:    limit,
		}

		resp, err := client.ListBeads(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadListJSON(resp.GetBeads())
		} else {
			printBeadListTable(resp.GetBeads(), resp.GetTotal())
		}
		return nil
	},
}

func init() {
	blockedCmd.Flags().String("assignee", "", "filter by assignee")
	blockedCmd.Flags().StringSliceP("type", "t", nil, "filter by type (repeatable)")
	blockedCmd.Flags().Int32("limit", 20, "maximum number of results")
}
