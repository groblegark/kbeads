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

// contextConfig is the client-side interpretation of a context:{name} config value.
type contextConfig struct {
	Sections []contextSection `json:"sections"`
}

type contextSection struct {
	Header string   `json:"header"`
	View   string   `json:"view"`
	Format string   `json:"format"` // "table" (default), "list", "count"
	Fields []string `json:"fields"` // for "list" format
}

var contextCmd = &cobra.Command{
	Use:   "context <name>",
	Short: "Compose and render a context template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// 1. Fetch the context config.
		config, err := beadsClient.GetConfig(context.Background(), "context:"+name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var cc contextConfig
		if err := json.Unmarshal(config.Value, &cc); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing context config: %v\n", err)
			os.Exit(1)
		}

		// 2. Render each section.
		for i, section := range cc.Sections {
			if i > 0 {
				fmt.Println()
			}
			if section.Header != "" {
				fmt.Println(section.Header)
				fmt.Println()
			}

			// Resolve the named view.
			viewCfg, err := beadsClient.GetConfig(context.Background(), "view:"+section.View)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading view %q: %v\n", section.View, err)
				continue
			}

			var vc viewConfig
			if err := json.Unmarshal(viewCfg.Value, &vc); err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing view %q: %v\n", section.View, err)
				continue
			}

			// Build and execute the query.
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

			resp, err := beadsClient.ListBeads(context.Background(), req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error querying view %q: %v\n", section.View, err)
				continue
			}

			// Format output.
			switch section.Format {
			case "count":
				fmt.Printf("%d beads\n", resp.Total)
			case "list":
				printSectionList(resp.Beads, section.Fields)
			default: // "table" or empty
				if len(vc.Columns) > 0 {
					printBeadListColumns(resp.Beads, resp.Total, vc.Columns)
				} else {
					printSectionTable(resp.Beads)
				}
			}
		}
		return nil
	},
}

// printSectionList prints beads as bullet points with selected fields.
func printSectionList(beads []*model.Bead, fields []string) {
	if len(fields) == 0 {
		fields = []string{"id", "title", "status"}
	}
	for _, b := range beads {
		parts := make([]string, len(fields))
		for i, f := range fields {
			parts[i] = beadField(b, f)
		}
		fmt.Printf("- %s\n", strings.Join(parts, " | "))
	}
}

// printSectionTable prints beads as a compact table (no total footer).
func printSectionTable(beads []*model.Bead) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tTYPE\tPRIORITY\tTITLE\tASSIGNEE")
	for _, b := range beads {
		title := b.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			b.ID,
			b.Status,
			b.Type,
			b.Priority,
			title,
			b.Assignee,
		)
	}
	w.Flush()
}
