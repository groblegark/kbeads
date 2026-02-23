package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// viewConfig is the client-side interpretation of a view:{name} config value.
type viewConfig struct {
	Filter  viewFilter `json:"filter"`
	Sort    string     `json:"sort"`
	Columns []string   `json:"columns"`
	Limit   int32      `json:"limit"`
}

type viewFilter struct {
	Status   []string `json:"status"`
	Type     []string `json:"type"`
	Kind     []string `json:"kind"`
	Labels   []string `json:"labels"`
	Assignee string   `json:"assignee"`
	Search   string   `json:"search"`
	Priority *int32   `json:"priority"`
}

var viewCmd = &cobra.Command{
	Use:   "view <name>",
	Short: "Run a saved view (named query)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		limitOverride, _ := cmd.Flags().GetInt32("limit")

		// 1. Fetch the view config.
		resp, err := client.GetConfig(context.Background(), &beadsv1.GetConfigRequest{
			Key: "view:" + name,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var vc viewConfig
		if err := json.Unmarshal(resp.GetConfig().GetValue(), &vc); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing view config: %v\n", err)
			os.Exit(1)
		}

		// 2. Build the ListBeads request.
		req := &beadsv1.ListBeadsRequest{
			Status:   vc.Filter.Status,
			Type:     vc.Filter.Type,
			Kind:     vc.Filter.Kind,
			Labels:   vc.Filter.Labels,
			Assignee: expandVar(vc.Filter.Assignee),
			Search:   vc.Filter.Search,
			Sort:     vc.Sort,
			Limit:    vc.Limit,
		}
		if vc.Filter.Priority != nil {
			req.Priority = wrapperspb.Int32(*vc.Filter.Priority)
		}
		if limitOverride > 0 {
			req.Limit = limitOverride
		}

		// 3. Call ListBeads.
		listResp, err := client.ListBeads(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// 4. Display results.
		if jsonOutput {
			printBeadListJSON(listResp.GetBeads())
		} else if len(vc.Columns) > 0 {
			printBeadListColumns(listResp.GetBeads(), listResp.GetTotal(), vc.Columns)
		} else {
			printBeadListTable(listResp.GetBeads(), listResp.GetTotal())
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
func printBeadListColumns(beads []*beadsv1.Bead, total int32, columns []string) {
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
func beadField(b *beadsv1.Bead, col string) string {
	switch strings.ToLower(col) {
	case "id":
		return b.GetId()
	case "title":
		title := b.GetTitle()
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		return title
	case "status":
		return b.GetStatus()
	case "type":
		return b.GetType()
	case "kind":
		return b.GetKind()
	case "priority":
		return fmt.Sprintf("%d", b.GetPriority())
	case "assignee":
		return b.GetAssignee()
	case "owner":
		return b.GetOwner()
	case "created_by":
		return b.GetCreatedBy()
	case "labels":
		return strings.Join(b.GetLabels(), ",")
	default:
		return ""
	}
}

func init() {
	viewCmd.Flags().Int32("limit", 0, "override the view's limit")
}
