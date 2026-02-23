package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/groblegark/kbeads/internal/model"
)

func printBeadJSON(bead *model.Bead) {
	data, err := json.MarshalIndent(bead, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printBeadTable(bead *model.Bead) {
	fmt.Printf("ID:          %s\n", bead.ID)
	fmt.Printf("Slug:        %s\n", bead.Slug)
	fmt.Printf("Title:       %s\n", bead.Title)
	fmt.Printf("Type:        %s\n", bead.Type)
	fmt.Printf("Kind:        %s\n", bead.Kind)
	fmt.Printf("Status:      %s\n", bead.Status)
	fmt.Printf("Priority:    %d\n", bead.Priority)
	fmt.Printf("Assignee:    %s\n", bead.Assignee)
	fmt.Printf("Owner:       %s\n", bead.Owner)
	if bead.Description != "" {
		fmt.Printf("Description: %s\n", bead.Description)
	}
	if len(bead.Labels) > 0 {
		fmt.Printf("Labels:      %s\n", strings.Join(bead.Labels, ", "))
	}
	fmt.Printf("Created By:  %s\n", bead.CreatedBy)
	if !bead.CreatedAt.IsZero() {
		fmt.Printf("Created At:  %s\n", bead.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	if !bead.UpdatedAt.IsZero() {
		fmt.Printf("Updated At:  %s\n", bead.UpdatedAt.Format("2006-01-02 15:04:05"))
	}
}

func printBeadListJSON(beads []*model.Bead) {
	data, err := json.MarshalIndent(beads, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printBeadListTable(beads []*model.Bead, total int) {
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
	fmt.Printf("\n%d beads (%d total)\n", len(beads), total)
}
