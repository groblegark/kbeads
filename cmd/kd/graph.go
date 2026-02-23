package main

import (
	"context"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph <bead-id>",
	Short: "Show dependency graph as ASCII tree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		depth, _ := cmd.Flags().GetInt("depth")

		// Fetch the root bead
		bead, err := beadsClient.GetBead(context.Background(), beadID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("%s [%s] %s\n", bead.ID, bead.Status, bead.Title)

		// Get dependencies and print tree
		printDepTree(bead.Dependencies, "", depth-1)

		return nil
	},
}

func printDepTree(deps []*model.Dependency, prefix string, remainingDepth int) {
	for i, dep := range deps {
		isLast := i == len(deps)-1

		// Choose connector
		connector := "\u251c\u2500\u2500 "
		childPrefix := prefix + "\u2502   "
		if isLast {
			connector = "\u2514\u2500\u2500 "
			childPrefix = prefix + "    "
		}

		// Fetch the dependent bead to get its title and status
		depBead, err := beadsClient.GetBead(context.Background(), dep.DependsOnID)
		if err != nil {
			fmt.Printf("%s%s%s: %s (error fetching)\n", prefix, connector, dep.Type, dep.DependsOnID)
			continue
		}

		fmt.Printf("%s%s%s: %s [%s] %s\n",
			prefix, connector,
			dep.Type,
			depBead.ID,
			depBead.Status,
			depBead.Title,
		)

		// Recurse if we have remaining depth
		if remainingDepth > 0 {
			if len(depBead.Dependencies) > 0 {
				printDepTree(depBead.Dependencies, childPrefix, remainingDepth-1)
			}
		}
	}
}

func init() {
	graphCmd.Flags().Int("depth", 3, "maximum depth to traverse")
}
