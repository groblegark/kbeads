package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var unclaimCmd = &cobra.Command{
	Use:   "unclaim <id>...",
	Short: "Unclaim one or more beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, id := range args {
			resp, err := client.UpdateBead(context.Background(), &beadsv1.UpdateBeadRequest{
				Id:       id,
				Assignee: proto.String(""),
				Status:   proto.String("open"),
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error unclaiming %s: %v\n", id, err)
				os.Exit(1)
			}

			if jsonOutput {
				printBeadJSON(resp.GetBead())
			} else {
				if len(args) > 1 {
					fmt.Printf("Unclaimed %s\n", resp.GetBead().GetId())
				} else {
					printBeadTable(resp.GetBead())
				}
			}
		}
		return nil
	},
}
