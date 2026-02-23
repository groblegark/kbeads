package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var adviceRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an advice bead",
	Long:  "Close (or delete) an advice bead by ID.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		hard, _ := cmd.Flags().GetBool("hard")
		if hard {
			if err := beadsClient.DeleteBead(context.Background(), id); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Deleted advice %s\n", id)
		} else {
			bead, err := beadsClient.CloseBead(context.Background(), id, actor)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Closed advice %s: %s\n", bead.ID, bead.Title)
		}

		return nil
	},
}

func init() {
	adviceRemoveCmd.Flags().Bool("hard", false, "permanently delete instead of closing")
}
