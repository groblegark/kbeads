package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List templates",
	Long: `List available templates.

Examples:
  kd template list
  kd template list --label project:gasboat
  kd template list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		labels, _ := cmd.Flags().GetStringSlice("label")

		req := &client.ListBeadsRequest{
			Type:   []string{"template"},
			Status: []string{"open", "in_progress"},
			Labels: labels,
			Limit:  100,
		}

		resp, err := beadsClient.ListBeads(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing templates: %w", err)
		}

		if jsonOutput {
			printBeadListJSON(resp.Beads)
		} else {
			printTemplateList(resp.Beads, resp.Total)
		}
		return nil
	},
}

func printTemplateList(beads []*model.Bead, total int) {
	if len(beads) == 0 {
		fmt.Println("No templates found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTEPS\tVARS\tLABELS")
	for _, b := range beads {
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		steps, vars := templateFieldCounts(b)
		lblStr := strings.Join(b.Labels, ", ")

		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n", b.ID, title, steps, vars, lblStr)
	}
	w.Flush()
	fmt.Printf("\n%d templates (%d total)\n", len(beads), total)
}

// templateFieldCounts extracts step and var counts from a template bead's fields.
func templateFieldCounts(b *model.Bead) (steps int, vars int) {
	if len(b.Fields) == 0 {
		return 0, 0
	}
	var f struct {
		Steps []json.RawMessage `json:"steps"`
		Vars  []json.RawMessage `json:"vars"`
	}
	if json.Unmarshal(b.Fields, &f) == nil {
		steps = len(f.Steps)
		vars = len(f.Vars)
	}
	return
}

func init() {
	templateListCmd.Flags().StringSliceP("label", "l", nil, "filter by label (repeatable)")
}
