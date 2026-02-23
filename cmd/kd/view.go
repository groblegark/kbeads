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

// viewConfig is the client-side interpretation of a view:{name} config value.
type viewConfig struct {
	Filter  viewFilter `json:"filter"`
	Sort    string     `json:"sort"`
	Columns []string   `json:"columns"`
	Limit   int        `json:"limit"`
}

type viewFilter struct {
	Status   []string `json:"status"`
	Type     []string `json:"type"`
	Kind     []string `json:"kind"`
	Labels   []string `json:"labels"`
	Assignee string   `json:"assignee"`
	Search   string   `json:"search"`
	Priority *int     `json:"priority"`
}

var viewCmd = &cobra.Command{
	Use:   "view <name>",
	Short: "Run a saved view (named query)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		limitOverride, _ := cmd.Flags().GetInt("limit")

		// 1. Fetch the view config.
		config, err := beadsClient.GetConfig(context.Background(), "view:"+name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var vc viewConfig
		if err := json.Unmarshal(config.Value, &vc); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing view config: %v\n", err)
			os.Exit(1)
		}

		// 2. Build the ListBeads request.
		req := &client.ListBeadsRequest{
			Status:   vc.Filter.Status,
			Type:     vc.Filter.Type,
			Kind:     vc.Filter.Kind,
			Labels:   vc.Filter.Labels,
			Assignee: expandVar(vc.Filter.Assignee),
			Search:   vc.Filter.Search,
			Sort:     vc.Sort,
			Limit:    vc.Limit,
			Priority: vc.Filter.Priority,
		}
		if limitOverride > 0 {
			req.Limit = limitOverride
		}

		// 3. Call ListBeads.
		resp, err := beadsClient.ListBeads(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// 4. Display results.
		if jsonOutput {
			printBeadListJSON(resp.Beads)
		} else if len(vc.Columns) > 0 {
			printBeadListColumns(resp.Beads, resp.Total, vc.Columns)
		} else {
			printBeadListTable(resp.Beads, resp.Total)
		}
		return nil
	},
}

// expandVar replaces well-known variables in filter values.
func expandVar(s string) string {
	s = strings.ReplaceAll(s, "$BEADS_ACTOR", actor)
	return s
}

// printBeadListColumns prints beads using a custom set of columns.
func printBeadListColumns(beads []*model.Bead, total int, columns []string) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	// Header
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = strings.ToUpper(c)
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Rows
	for _, b := range beads {
		vals := make([]string, len(columns))
		for i, col := range columns {
			vals[i] = beadField(b, col)
		}
		fmt.Fprintln(tw, strings.Join(vals, "\t"))
	}
	tw.Flush()
	fmt.Printf("\n%d beads (%d total)\n", len(beads), total)
}

// beadField returns the string value of a bead field by column name.
func beadField(b *model.Bead, col string) string {
	switch strings.ToLower(col) {
	case "id":
		return b.ID
	case "title":
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		return title
	case "status":
		return string(b.Status)
	case "type":
		return string(b.Type)
	case "kind":
		return string(b.Kind)
	case "priority":
		return fmt.Sprintf("%d", b.Priority)
	case "assignee":
		return b.Assignee
	case "owner":
		return b.Owner
	case "created_by":
		return b.CreatedBy
	case "labels":
		return strings.Join(b.Labels, ",")
	default:
		return ""
	}
}

func init() {
	viewCmd.Flags().Int("limit", 0, "override the view's limit")
}
