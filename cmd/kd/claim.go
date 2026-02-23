package main

import (
	"context"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

var claimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a bead by assigning it to yourself",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		inProgress := "in_progress"

		bead, err := beadsClient.UpdateBead(context.Background(), id, &client.UpdateBeadRequest{
			Assignee: &actor,
			Status:   &inProgress,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			printBeadTable(bead)
		}
		return nil
	},
}
