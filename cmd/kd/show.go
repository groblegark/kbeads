package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of a bead",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		bead, err := beadsClient.GetBead(context.Background(), id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			printBeadTable(bead)
			if len(bead.Comments) > 0 {
				fmt.Println()
				fmt.Println("Comments:")
				for _, c := range bead.Comments {
					ts := c.CreatedAt.Format("2006-01-02 15:04:05")
					fmt.Printf("  [%s] %s: %s\n", ts, c.Author, c.Text)
				}
			}
		}
		return nil
	},
}
