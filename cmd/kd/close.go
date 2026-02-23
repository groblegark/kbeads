package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var closeCmd = &cobra.Command{
	Use:   "close <id>...",
	Short: "Close one or more beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, id := range args {
			resp, err := client.CloseBead(context.Background(), &beadsv1.CloseBeadRequest{
				Id:       id,
				ClosedBy: actor,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error closing %s: %v\n", id, err)
				os.Exit(1)
			}

			if jsonOutput {
				printBeadJSON(resp.GetBead())
			} else {
				if len(args) > 1 {
					fmt.Printf("Closed %s\n", resp.GetBead().GetId())
				} else {
					printBeadTable(resp.GetBead())
				}
			}
		}
		return nil
	},
}
