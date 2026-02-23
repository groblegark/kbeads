package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
)

func printBeadJSON(bead *beadsv1.Bead) {
	data, err := json.MarshalIndent(bead, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printBeadTable(bead *beadsv1.Bead) {
	fmt.Printf("ID:          %s\n", bead.GetId())
	fmt.Printf("Slug:        %s\n", bead.GetSlug())
	fmt.Printf("Title:       %s\n", bead.GetTitle())
	fmt.Printf("Type:        %s\n", bead.GetType())
	fmt.Printf("Kind:        %s\n", bead.GetKind())
	fmt.Printf("Status:      %s\n", bead.GetStatus())
	fmt.Printf("Priority:    %d\n", bead.GetPriority())
	fmt.Printf("Assignee:    %s\n", bead.GetAssignee())
	fmt.Printf("Owner:       %s\n", bead.GetOwner())
	if bead.GetDescription() != "" {
		fmt.Printf("Description: %s\n", bead.GetDescription())
	}
	if len(bead.GetLabels()) > 0 {
		fmt.Printf("Labels:      %s\n", strings.Join(bead.GetLabels(), ", "))
	}
	fmt.Printf("Created By:  %s\n", bead.GetCreatedBy())
	if bead.GetCreatedAt() != nil {
		fmt.Printf("Created At:  %s\n", bead.GetCreatedAt().AsTime().Format("2006-01-02 15:04:05"))
	}
	if bead.GetUpdatedAt() != nil {
		fmt.Printf("Updated At:  %s\n", bead.GetUpdatedAt().AsTime().Format("2006-01-02 15:04:05"))
	}
}

func printBeadListJSON(beads []*beadsv1.Bead) {
	data, err := json.MarshalIndent(beads, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printBeadListTable(beads []*beadsv1.Bead, total int32) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tTYPE\tPRIORITY\tTITLE\tASSIGNEE")
	for _, b := range beads {
		title := b.GetTitle()
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			b.GetId(),
			b.GetStatus(),
			b.GetType(),
			b.GetPriority(),
			title,
			b.GetAssignee(),
		)
	}
	w.Flush()
	fmt.Printf("\n%d beads (%d total)\n", len(beads), total)
}
