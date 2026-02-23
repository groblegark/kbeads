package main

import (
	"context"
	"fmt"
	"os"

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
				_, err := beadsClient.AddComment(context.Background(), id, actor, comment)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error adding comment to %s: %v\n", id, err)
					os.Exit(1)
				}
			}

			bead, err := beadsClient.CloseBead(context.Background(), id, actor)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error closing %s: %v\n", id, err)
				os.Exit(1)
			}

			if jsonOutput {
				printBeadJSON(bead)
			} else {
				if len(args) > 1 {
					fmt.Printf("Done %s\n", bead.ID)
				} else {
					printBeadTable(bead)
				}
			}
		}
		return nil
	},
}

func init() {
	doneCmd.Flags().StringP("comment", "m", "", "completion comment to add before closing")
}
