package main

import (
	"context"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

// parseFields converts -f key=value pairs into a JSON object (bytes).
// Values that look like JSON (start with { [ " or are true/false/null/number)
// are embedded as-is; everything else is quoted as a string.
func parseFields(pairs []string) ([]byte, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	m := make(map[string]any, len(pairs))
	for _, p := range pairs {
		k, v, ok := splitField(p)
		if !ok {
			return nil, fmt.Errorf("invalid field %q: expected key=value", p)
		}
		m[k] = rawOrString(v)
	}
	b, err := jsonMarshal(m)
	if err != nil {
		return nil, fmt.Errorf("encoding fields: %w", err)
	}
	return b, nil
}

var createCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new bead",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

		description, _ := cmd.Flags().GetString("description")
		beadType, _ := cmd.Flags().GetString("type")
		kind, _ := cmd.Flags().GetString("kind")
		priority, _ := cmd.Flags().GetInt("priority")
		labels, _ := cmd.Flags().GetStringSlice("label")
		assignee, _ := cmd.Flags().GetString("assignee")
		owner, _ := cmd.Flags().GetString("owner")

		fieldPairs, _ := cmd.Flags().GetStringArray("field")
		fieldsJSON, err := parseFields(fieldPairs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		req := &client.CreateBeadRequest{
			Title:       title,
			Description: description,
			Type:        beadType,
			Kind:        kind,
			Priority:    priority,
			Labels:      labels,
			Assignee:    assignee,
			Owner:       owner,
			CreatedBy:   actor,
			Fields:      fieldsJSON,
		}

		bead, err := beadsClient.CreateBead(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			printBeadTable(bead)
		}
		return nil
	},
}

func init() {
	createCmd.Flags().StringP("description", "d", "", "bead description")
	createCmd.Flags().StringP("type", "t", "task", "bead type")
	createCmd.Flags().StringP("kind", "k", "", "bead kind (optional, inferred from type)")
	createCmd.Flags().IntP("priority", "p", 2, "bead priority")
	createCmd.Flags().StringSliceP("label", "l", nil, "labels (repeatable)")
	createCmd.Flags().String("assignee", "", "assignee")
	createCmd.Flags().String("owner", "", "owner")
	createCmd.Flags().StringArrayP("field", "f", nil, "typed field (key=value, repeatable)")
}
