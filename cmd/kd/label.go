package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "Manage bead labels",
}

var labelAddCmd = &cobra.Command{
	Use:   "add <bead-id> <label>...",
	Short: "Add labels to a bead",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		labels := args[1:]

		for _, label := range labels {
			_, err := client.AddLabel(context.Background(), &beadsv1.AddLabelRequest{
				BeadId: beadID,
				Label:  label,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error adding label %q: %v\n", label, err)
				os.Exit(1)
			}
		}

		fmt.Printf("Added label(s) to %s\n", beadID)
		return nil
	},
}

var labelRemoveCmd = &cobra.Command{
	Use:   "remove <bead-id> <label>...",
	Short: "Remove labels from a bead",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		labels := args[1:]

		for _, label := range labels {
			_, err := client.RemoveLabel(context.Background(), &beadsv1.RemoveLabelRequest{
				BeadId: beadID,
				Label:  label,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error removing label %q: %v\n", label, err)
				os.Exit(1)
			}
		}

		fmt.Printf("Removed label(s) from %s\n", beadID)
		return nil
	},
}

func init() {
	labelCmd.AddCommand(labelAddCmd)
	labelCmd.AddCommand(labelRemoveCmd)
}
