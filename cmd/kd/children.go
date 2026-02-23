package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var childrenCmd = &cobra.Command{
	Use:   "children <parent-id>",
	Short: "List dependencies of a bead (forward dependencies)",
	Long: `List dependencies of a bead. Shows the forward dependencies
(things this bead depends on) filtered by type.

Note: This shows forward dependencies (bead_id -> depends_on_id).
To find beads that depend ON this bead (reverse lookup), that
would require querying all beads, which is not yet supported
as a dedicated RPC.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parentID := args[0]
		filterType, _ := cmd.Flags().GetString("type")

		deps, err := beadsClient.GetDependencies(context.Background(), parentID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Filter dependencies by type if specified
		var filtered []*model.Dependency
		for _, d := range deps {
			if filterType == "" || string(d.Type) == filterType {
				filtered = append(filtered, d)
			}
		}

		if len(filtered) == 0 {
			fmt.Println("No dependencies found.")
			return nil
		}

		// Fetch each dependent bead for display
		type childInfo struct {
			dep  *model.Dependency
			bead *model.Bead
		}
		var children []childInfo
		for _, d := range filtered {
			bead, err := beadsClient.GetBead(context.Background(), d.DependsOnID)
			if err != nil {
				children = append(children, childInfo{dep: d, bead: nil})
				continue
			}
			children = append(children, childInfo{dep: d, bead: bead})
		}

		if jsonOutput {
			// Build a JSON-friendly structure
			type jsonChild struct {
				DependsOnID string `json:"depends_on_id"`
				Type        string `json:"type"`
				Status      string `json:"status,omitempty"`
				Title       string `json:"title,omitempty"`
			}
			var out []jsonChild
			for _, c := range children {
				jc := jsonChild{
					DependsOnID: c.dep.DependsOnID,
					Type:        string(c.dep.Type),
				}
				if c.bead != nil {
					jc.Status = string(c.bead.Status)
					jc.Title = c.bead.Title
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
			for _, c := range children {
				status := "(unknown)"
				title := "(error fetching)"
				if c.bead != nil {
					status = string(c.bead.Status)
					title = c.bead.Title
					if len(title) > 50 {
						title = title[:47] + "..."
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					c.dep.DependsOnID,
					c.dep.Type,
					status,
					title,
				)
			}
			w.Flush()
		}
		return nil
	},
}

func init() {
	childrenCmd.Flags().StringP("type", "t", "", "filter by dependency type (e.g. parent-child, blocks)")
}
