package main

import (
	"context"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

var reopenCmd = &cobra.Command{
	Use:   "reopen <id>...",
	Short: "Reopen one or more closed beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		open := "open"
		for _, id := range args {
			bead, err := beadsClient.UpdateBead(context.Background(), id, &client.UpdateBeadRequest{
				Status: &open,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reopening %s: %v\n", id, err)
				os.Exit(1)
			}

			if jsonOutput {
				printBeadJSON(bead)
			} else {
				if len(args) > 1 {
					fmt.Printf("Reopened %s\n", bead.ID)
				} else {
					printBeadTable(bead)
				}
			}
		}
		return nil
	},
}
