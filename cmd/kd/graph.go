package main

import (
	"context"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
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
		resp, err := client.GetBead(context.Background(), &beadsv1.GetBeadRequest{
			Id: beadID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		bead := resp.GetBead()
		fmt.Printf("%s [%s] %s\n", bead.GetId(), bead.GetStatus(), bead.GetTitle())

		// Get dependencies and print tree
		deps := bead.GetDependencies()
		printDepTree(deps, "", depth-1)

		return nil
	},
}

func printDepTree(deps []*beadsv1.Dependency, prefix string, remainingDepth int) {
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
		depResp, err := client.GetBead(context.Background(), &beadsv1.GetBeadRequest{
			Id: dep.GetDependsOnId(),
		})
		if err != nil {
			fmt.Printf("%s%s%s: %s (error fetching)\n", prefix, connector, dep.GetType(), dep.GetDependsOnId())
			continue
		}

		depBead := depResp.GetBead()
		fmt.Printf("%s%s%s: %s [%s] %s\n",
			prefix, connector,
			dep.GetType(),
			depBead.GetId(),
			depBead.GetStatus(),
			depBead.GetTitle(),
		)

		// Recurse if we have remaining depth
		if remainingDepth > 0 {
			childDeps := depBead.GetDependencies()
			if len(childDeps) > 0 {
				printDepTree(childDeps, childPrefix, remainingDepth-1)
			}
		}
	}
}

func init() {
	graphCmd.Flags().Int("depth", 3, "maximum depth to traverse")
}
