package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var claimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a bead by assigning it to yourself",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		resp, err := client.UpdateBead(context.Background(), &beadsv1.UpdateBeadRequest{
			Id:       id,
			Assignee: proto.String(actor),
			Status:   proto.String("in_progress"),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(resp.GetBead())
		} else {
			printBeadTable(resp.GetBead())
		}
		return nil
	},
}
