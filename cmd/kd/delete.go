package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <id>...",
	Short: "Delete one or more beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, id := range args {
			_, err := client.DeleteBead(context.Background(), &beadsv1.DeleteBeadRequest{
				Id: id,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error deleting %s: %v\n", id, err)
				os.Exit(1)
			}

			fmt.Printf("Deleted %s\n", id)
		}
		return nil
	},
}
