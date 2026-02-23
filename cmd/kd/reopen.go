package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var reopenCmd = &cobra.Command{
	Use:   "reopen <id>...",
	Short: "Reopen one or more closed beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, id := range args {
			resp, err := client.UpdateBead(context.Background(), &beadsv1.UpdateBeadRequest{
				Id:     id,
				Status: proto.String("open"),
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reopening %s: %v\n", id, err)
				os.Exit(1)
			}

			if jsonOutput {
				printBeadJSON(resp.GetBead())
			} else {
				if len(args) > 1 {
					fmt.Printf("Reopened %s\n", resp.GetBead().GetId())
				} else {
					printBeadTable(resp.GetBead())
				}
			}
		}
		return nil
	},
}
