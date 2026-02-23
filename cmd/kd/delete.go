package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <id>...",
	Short: "Delete one or more beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, id := range args {
			if err := beadsClient.DeleteBead(context.Background(), id); err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", id, err)
				os.Exit(1)
			}

			fmt.Printf("Deleted %s\n", id)
		}
		return nil
	},
}
