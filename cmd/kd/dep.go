package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage bead dependencies",
}

var depAddCmd = &cobra.Command{
	Use:   "add <bead-id> <depends-on-id>",
	Short: "Add a dependency between beads",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		dependsOnID := args[1]
		depType, _ := cmd.Flags().GetString("type")

		resp, err := client.AddDependency(context.Background(), &beadsv1.AddDependencyRequest{
			BeadId:      beadID,
			DependsOnId: dependsOnID,
			Type:        depType,
			CreatedBy:   actor,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		dep := resp.GetDependency()
		if jsonOutput {
			data, err := json.MarshalIndent(dep, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			fmt.Printf("Bead:        %s\n", dep.GetBeadId())
			fmt.Printf("Depends On:  %s\n", dep.GetDependsOnId())
			fmt.Printf("Type:        %s\n", dep.GetType())
			fmt.Printf("Created By:  %s\n", dep.GetCreatedBy())
			if dep.GetCreatedAt() != nil {
				fmt.Printf("Created At:  %s\n", dep.GetCreatedAt().AsTime().Format("2006-01-02 15:04:05"))
			}
		}
		return nil
	},
}

var depRemoveCmd = &cobra.Command{
	Use:   "remove <bead-id> <depends-on-id>",
	Short: "Remove a dependency between beads",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		dependsOnID := args[1]
		depType, _ := cmd.Flags().GetString("type")

		_, err := client.RemoveDependency(context.Background(), &beadsv1.RemoveDependencyRequest{
			BeadId:      beadID,
			DependsOnId: dependsOnID,
			Type:        depType,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Removed dependency")
		return nil
	},
}

var depListCmd = &cobra.Command{
	Use:   "list <bead-id>",
	Short: "List dependencies of a bead",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]

		resp, err := client.GetDependencies(context.Background(), &beadsv1.GetDependenciesRequest{
			BeadId: beadID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		deps := resp.GetDependencies()
		if jsonOutput {
			data, err := json.MarshalIndent(deps, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			if len(deps) == 0 {
				fmt.Println("No dependencies found.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DEPENDS_ON\tTYPE\tCREATED_BY\tCREATED_AT")
			for _, d := range deps {
				createdAt := ""
				if d.GetCreatedAt() != nil {
					createdAt = d.GetCreatedAt().AsTime().Format("2006-01-02 15:04:05")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					d.GetDependsOnId(),
					d.GetType(),
					d.GetCreatedBy(),
					createdAt,
				)
			}
			w.Flush()
		}
		return nil
	},
}

func init() {
	depAddCmd.Flags().StringP("type", "t", "blocks", "dependency type")
	depRemoveCmd.Flags().StringP("type", "t", "blocks", "dependency type")

	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
	depCmd.AddCommand(depListCmd)
}
