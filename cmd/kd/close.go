package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var closeCmd = &cobra.Command{
	Use:   "close <id>...",
	Short: "Close one or more beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, id := range args {
			bead, err := beadsClient.CloseBead(context.Background(), id, actor)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error closing %s: %v\n", id, err)
				os.Exit(1)
			}

			if jsonOutput {
				printBeadJSON(bead)
			} else {
				if len(args) > 1 {
					fmt.Printf("Closed %s\n", bead.ID)
				} else {
					printBeadTable(bead)
				}
			}
		}
		return nil
	},
}
