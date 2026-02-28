package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:     "tree <bead-id>",
	Short:   "Show dependency tree (or flat list) for a bead",
	GroupID: "views",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		beadID := args[0]
		depth, _ := cmd.Flags().GetInt("depth")
		flat, _ := cmd.Flags().GetBool("flat")
		filterType, _ := cmd.Flags().GetString("type")
		reverse, _ := cmd.Flags().GetBool("reverse")

		if flat {
			if reverse {
				return runReverseTreeFlat(beadID, filterType)
			}
			return runTreeFlat(beadID, filterType)
		}
		if reverse {
			return runReverseTreeGraph(beadID, depth, filterType)
		}
		return runTreeGraph(beadID, depth, filterType)
	},
}

func runTreeGraph(beadID string, depth int, filterType string) error {
	bead, err := beadsClient.GetBead(context.Background(), beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}


	fmt.Printf("%s [%s] %s\n", bead.ID, string(bead.Status), bead.Title)

	deps := bead.Dependencies
	if filterType != "" {
		deps = filterDepsByType(deps, []string{filterType})
	}
	printDepTree(deps, "", depth-1)
	return nil
}

func runTreeFlat(beadID string, filterType string) error {
	var types []string
	if filterType != "" {
		types = []string{filterType}
	}

	deps, err := fetchAndResolveDeps(context.Background(), beadsClient, beadID, types)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(deps) == 0 {
		fmt.Println("No dependencies found.")
		return nil
	}

	if jsonOutput {
		type jsonChild struct {
			DependsOnID string `json:"depends_on_id"`
			Type        string `json:"type"`
			Status      string `json:"status,omitempty"`
			Title       string `json:"title,omitempty"`
		}
		var out []jsonChild
		for _, rd := range deps {
			jc := jsonChild{
				DependsOnID: rd.Dep.DependsOnID,
				Type:        string(rd.Dep.Type),
			}
			if rd.Bead != nil {
				jc.Status = string(rd.Bead.Status)
				jc.Title = rd.Bead.Title
			}
			out = append(out, jc)
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DEPENDS_ON\tTYPE\tSTATUS\tTITLE")
		for _, rd := range deps {
			status := "(unknown)"
			title := "(error fetching)"
			if rd.Bead != nil {
				status = string(rd.Bead.Status)
				title = rd.Bead.Title
				if len(title) > 50 {
					title = title[:47] + "..."
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				rd.Dep.DependsOnID,
				string(rd.Dep.Type),
				status,
				title,
			)
		}
		w.Flush()
	}
	return nil
}

// runReverseTreeGraph prints an ASCII tree of beads that depend ON the given bead.
func runReverseTreeGraph(beadID string, depth int, filterType string) error {
	bead, err := beadsClient.GetBead(context.Background(), beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s [%s] %s\n", bead.ID, string(bead.Status), bead.Title)

	deps, err := beadsClient.GetReverseDependencies(context.Background(), beadID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching reverse deps: %v\n", err)
		os.Exit(1)
	}
	if filterType != "" {
		deps = filterDepsByType(deps, []string{filterType})
	}
	printReverseDepTree(deps, "", depth-1)
	return nil
}

// runReverseTreeFlat prints a flat table of beads that depend ON the given bead.
func runReverseTreeFlat(beadID string, filterType string) error {
	deps, err := fetchAndResolveReverseDeps(context.Background(), beadsClient, beadID, filterType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(deps) == 0 {
		fmt.Println("No beads depend on this bead.")
		return nil
	}

	if jsonOutput {
		type jsonDependent struct {
			BeadID string `json:"bead_id"`
			Type   string `json:"type"`
			Status string `json:"status,omitempty"`
			Title  string `json:"title,omitempty"`
		}
		var out []jsonDependent
		for _, rd := range deps {
			jd := jsonDependent{
				BeadID: rd.Dep.BeadID,
				Type:   string(rd.Dep.Type),
			}
			if rd.Bead != nil {
				jd.Status = string(rd.Bead.Status)
				jd.Title = rd.Bead.Title
			}
			out = append(out, jd)
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "BEAD_ID\tTYPE\tSTATUS\tTITLE")
		for _, rd := range deps {
			status := "(unknown)"
			title := "(error fetching)"
			if rd.Bead != nil {
				status = string(rd.Bead.Status)
				title = rd.Bead.Title
				if len(title) > 50 {
					title = title[:47] + "..."
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				rd.Dep.BeadID,
				string(rd.Dep.Type),
				status,
				title,
			)
		}
		w.Flush()
	}
	return nil
}

func printDepTree(deps []*model.Dependency, prefix string, remainingDepth int) {
	for i, dep := range deps {
		isLast := i == len(deps)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		depBead, err := beadsClient.GetBead(context.Background(), dep.DependsOnID)
		if err != nil {
			fmt.Printf("%s%s%s: %s (error fetching)\n", prefix, connector, string(dep.Type), dep.DependsOnID)
			continue
		}


		fmt.Printf("%s%s%s: %s [%s] %s\n",
			prefix, connector,
			string(dep.Type),
			depBead.ID,
			string(depBead.Status),
			depBead.Title,
		)

		if remainingDepth > 0 {
			childDeps := depBead.Dependencies
			if len(childDeps) > 0 {
				printDepTree(childDeps, childPrefix, remainingDepth-1)
			}
		}
	}
}

// printReverseDepTree prints an ASCII tree of reverse dependents.
func printReverseDepTree(deps []*model.Dependency, prefix string, remainingDepth int) {
	for i, dep := range deps {
		isLast := i == len(deps)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		depBead, err := beadsClient.GetBead(context.Background(), dep.BeadID)
		if err != nil {
			fmt.Printf("%s%s%s: %s (error fetching)\n", prefix, connector, string(dep.Type), dep.BeadID)
			continue
		}

		fmt.Printf("%s%s%s: %s [%s] %s\n",
			prefix, connector,
			string(dep.Type),
			depBead.ID,
			string(depBead.Status),
			depBead.Title,
		)

		if remainingDepth > 0 {
			childDeps, err := beadsClient.GetReverseDependencies(context.Background(), dep.BeadID)
			if err == nil && len(childDeps) > 0 {
				printReverseDepTree(childDeps, childPrefix, remainingDepth-1)
			}
		}
	}
}

// fetchAndResolveReverseDeps fetches reverse dependencies and resolves each source bead.
func fetchAndResolveReverseDeps(ctx context.Context, c client.BeadsClient, beadID string, filterType string) ([]resolvedDep, error) {
	deps, err := c.GetReverseDependencies(ctx, beadID)
	if err != nil {
		return nil, err
	}
	var types []string
	if filterType != "" {
		types = []string{filterType}
	}
	return resolveReverseBeadDeps(ctx, c, deps, types), nil
}

// resolveReverseBeadDeps resolves the source bead for each reverse dependency.
func resolveReverseBeadDeps(ctx context.Context, c client.BeadsClient, deps []*model.Dependency, types []string) []resolvedDep {
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	var resolved []resolvedDep
	for _, d := range deps {
		if len(typeSet) > 0 && !typeSet[string(d.Type)] {
			continue
		}
		rd := resolvedDep{Dep: d}
		fetchedBead, err := c.GetBead(ctx, d.BeadID)
		if err == nil {
			rd.Bead = fetchedBead
		}
		resolved = append(resolved, rd)
	}
	return resolved
}

func init() {
	treeCmd.Flags().Int("depth", 3, "maximum depth to traverse")
	treeCmd.Flags().Bool("flat", false, "flat table instead of ASCII tree")
	treeCmd.Flags().StringP("type", "t", "", "filter by dependency type (e.g. parent-child, blocks)")
	treeCmd.Flags().Bool("reverse", false, "show beads that depend ON this bead (reverse lookup)")
}
