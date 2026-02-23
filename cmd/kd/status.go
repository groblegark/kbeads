package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database overview and bead counts by status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		statuses := []string{"open", "in_progress", "blocked", "deferred", "closed"}
		counts := make(map[string]int32, len(statuses))
		var total int32

		for _, s := range statuses {
			resp, err := client.ListBeads(ctx, &beadsv1.ListBeadsRequest{
				Status: []string{s},
				Limit:  0,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying %s beads: %v\n", s, err)
				os.Exit(1)
			}
			counts[s] = resp.GetTotal()
			total += resp.GetTotal()
		}

		if jsonOutput {
			out := map[string]int32{
				"open":        counts["open"],
				"in_progress": counts["in_progress"],
				"blocked":     counts["blocked"],
				"deferred":    counts["deferred"],
				"closed":      counts["closed"],
				"total":       total,
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(data))
		} else {
			fmt.Println("Beads Status")
			fmt.Printf("  Open:        %d\n", counts["open"])
			fmt.Printf("  In Progress: %d\n", counts["in_progress"])
			fmt.Printf("  Blocked:     %d\n", counts["blocked"])
			fmt.Printf("  Deferred:    %d\n", counts["deferred"])
			fmt.Printf("  Closed:      %d\n", counts["closed"])
			fmt.Printf("  Total:       %d\n", total)
		}
		return nil
	},
}
