package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

// TemplateVarDef defines a variable that a template accepts.
type TemplateVarDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     string   `json:"default,omitempty"`
	Type        string   `json:"type,omitempty"` // string, int, bool
	Enum        []string `json:"enum,omitempty"`
}

// TemplateStep defines a work item to create when a template is applied.
type TemplateStep struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"` // task, feature, bug, epic, chore
	Priority    *int     `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Condition   string   `json:"condition,omitempty"`
}

var templateCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a template from a JSON definition",
	Long: `Create a reusable template bead from a JSON file or inline JSON.

The template content is a JSON object with "vars" and "steps" arrays:

  {
    "vars": [
      {"name": "component", "description": "Component name", "required": true}
    ],
    "steps": [
      {"id": "design", "title": "Design {{component}}", "type": "task"},
      {"id": "implement", "title": "Implement {{component}}", "type": "task", "depends_on": ["design"]}
    ]
  }

Step titles and descriptions support {{variable}} substitution.

Examples:
  kd template create "Feature workflow" --file template.json
  kd template create "Bug fix" --file bugfix.json --label project:gasboat
  echo '{"steps":[...]}' | kd template create "Quick template" --file -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		filePath, _ := cmd.Flags().GetString("file")
		description, _ := cmd.Flags().GetString("description")
		priority, _ := cmd.Flags().GetInt("priority")
		labels, _ := cmd.Flags().GetStringSlice("label")

		if filePath == "" {
			return fmt.Errorf("--file is required: provide a JSON file path or - for stdin")
		}

		var data []byte
		var err error
		if filePath == "-" {
			data, err = os.ReadFile("/dev/stdin")
		} else {
			data, err = os.ReadFile(filePath)
		}
		if err != nil {
			return fmt.Errorf("reading template file: %w", err)
		}

		// Parse and validate the template content.
		var content struct {
			Vars  []TemplateVarDef `json:"vars"`
			Steps []TemplateStep   `json:"steps"`
		}
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("parsing template JSON: %w", err)
		}

		if len(content.Steps) == 0 {
			return fmt.Errorf("template must have at least one step")
		}

		// Validate step IDs are unique and depends_on references exist.
		stepIDs := make(map[string]bool, len(content.Steps))
		for _, s := range content.Steps {
			if s.ID == "" {
				return fmt.Errorf("every step must have an id")
			}
			if s.Title == "" {
				return fmt.Errorf("step %q must have a title", s.ID)
			}
			if stepIDs[s.ID] {
				return fmt.Errorf("duplicate step id %q", s.ID)
			}
			stepIDs[s.ID] = true
		}
		for _, s := range content.Steps {
			for _, dep := range s.DependsOn {
				if !stepIDs[dep] {
					return fmt.Errorf("step %q depends_on unknown step %q", s.ID, dep)
				}
			}
		}

		// Build the fields JSON.
		fields := map[string]any{
			"vars":  content.Vars,
			"steps": content.Steps,
		}
		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("encoding fields: %w", err)
		}

		req := &client.CreateBeadRequest{
			Title:       name,
			Description: description,
			Type:        "template",
			Priority:    priority,
			Labels:      labels,
			CreatedBy:   actor,
			Fields:      fieldsJSON,
		}

		bead, err := beadsClient.CreateBead(context.Background(), req)
		if err != nil {
			return fmt.Errorf("creating template: %w", err)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			fmt.Printf("Created template %s: %s\n", bead.ID, bead.Title)
			fmt.Printf("  Steps: %d\n", len(content.Steps))
			fmt.Printf("  Vars:  %d\n", len(content.Vars))
		}
		return nil
	},
}

func init() {
	templateCreateCmd.Flags().StringP("file", "f", "", "JSON file with template definition (use - for stdin)")
	templateCreateCmd.Flags().StringP("description", "d", "", "template description")
	templateCreateCmd.Flags().IntP("priority", "p", 2, "priority (0-4)")
	templateCreateCmd.Flags().StringSliceP("label", "l", nil, "labels (repeatable)")
}
