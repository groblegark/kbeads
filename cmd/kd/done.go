package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done <id>...",
	Short: "Mark beads as done and close them",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")

		for _, id := range args {
			if comment != "" {
				_, err := client.AddComment(context.Background(), &beadsv1.AddCommentRequest{
					BeadId: id,
					Author: actor,
					Text:   comment,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error adding comment to %s: %v\n", id, err)
					os.Exit(1)
				}
			}

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
					fmt.Printf("Done %s\n", resp.GetBead().GetId())
				} else {
					printBeadTable(resp.GetBead())
				}
			}
		}
		return nil
	},
}

func init() {
	doneCmd.Flags().StringP("comment", "m", "", "completion comment to add before closing")
}
