package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search beads by text query",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		status, _ := cmd.Flags().GetStringSlice("status")
		beadType, _ := cmd.Flags().GetStringSlice("type")
		kind, _ := cmd.Flags().GetStringSlice("kind")
		limit, _ := cmd.Flags().GetInt32("limit")

		req := &beadsv1.ListBeadsRequest{
			Search: query,
			Status: status,
			Type:   beadType,
			Kind:   kind,
			Limit:  limit,
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
	searchCmd.Flags().StringSliceP("status", "s", nil, "filter by status (repeatable)")
	searchCmd.Flags().StringSliceP("type", "t", nil, "filter by type (repeatable)")
	searchCmd.Flags().StringSliceP("kind", "k", nil, "filter by kind (repeatable)")
	searchCmd.Flags().Int32("limit", 20, "maximum number of results")
}
