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

var formulaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List formulas",
	Long: `List available formulas.

Examples:
  kd formula list
  kd formula list --label project:gasboat
  kd formula list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		labels, _ := cmd.Flags().GetStringSlice("label")

		req := &client.ListBeadsRequest{
			Type:   []string{"formula"},
			Status: []string{"open", "in_progress"},
			Labels: labels,
			Limit:  100,
		}

		resp, err := beadsClient.ListBeads(context.Background(), req)
		if err != nil {
			return fmt.Errorf("listing formulas: %w", err)
		}

		if jsonOutput {
			printBeadListJSON(resp.Beads)
		} else {
			printFormulaList(resp.Beads, resp.Total)
		}
		return nil
	},
}

func printFormulaList(beads []*model.Bead, total int) {
	if len(beads) == 0 {
		fmt.Println("No formulas found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTEPS\tVARS\tLABELS")
	for _, b := range beads {
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		steps, vars := formulaFieldCounts(b)
		lblStr := strings.Join(b.Labels, ", ")

		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n", b.ID, title, steps, vars, lblStr)
	}
	w.Flush()
	fmt.Printf("\n%d formulas (%d total)\n", len(beads), total)
}

// formulaFieldCounts extracts step and var counts from a formula bead's fields.
func formulaFieldCounts(b *model.Bead) (steps int, vars int) {
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
	formulaListCmd.Flags().StringSliceP("label", "l", nil, "filter by label (repeatable)")
}
