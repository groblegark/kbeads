package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

var templateApplyCmd = &cobra.Command{
	Use:   "apply <template-id>",
	Short: "Apply a template to create a bundle of beads",
	Long: `Apply a template to create a bundle (an epic with child beads).

The 3-step pipeline:
  1. Resolve — fetch the template and parse its vars/steps
  2. Expand  — substitute {{variables}} with provided values
  3. Create  — create the bundle bead and child issue beads

Examples:
  kd template apply kd-abc123 --var component=auth --var assignee=alice
  kd template apply kd-abc123 --var component=auth --dry-run
  kd template apply kd-abc123 --var component=auth --label project:gasboat`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		templateID := args[0]
		varPairs, _ := cmd.Flags().GetStringSlice("var")
		labels, _ := cmd.Flags().GetStringSlice("label")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		assignee, _ := cmd.Flags().GetString("assignee")

		ctx := context.Background()

		// ── Step 1: Resolve ─────────────────────────────────────
		bead, err := beadsClient.GetBead(ctx, templateID)
		if err != nil {
			return fmt.Errorf("resolving template: %w", err)
		}
		if string(bead.Type) != "template" {
			return fmt.Errorf("bead %s is type %q, not template", templateID, bead.Type)
		}

		var fields struct {
			Vars  []TemplateVarDef `json:"vars"`
			Steps []TemplateStep   `json:"steps"`
		}
		if len(bead.Fields) == 0 {
			return fmt.Errorf("template %s has no fields (empty template)", templateID)
		}
		if err := json.Unmarshal(bead.Fields, &fields); err != nil {
			return fmt.Errorf("parsing template fields: %w", err)
		}
		if len(fields.Steps) == 0 {
			return fmt.Errorf("template %s has no steps", templateID)
		}

		// Parse --var key=value pairs.
		vars := make(map[string]string, len(varPairs))
		for _, p := range varPairs {
			k, v, ok := splitField(p)
			if !ok {
				return fmt.Errorf("invalid --var %q: expected key=value", p)
			}
			vars[k] = v
		}

		// Apply defaults and validate required vars.
		for _, vd := range fields.Vars {
			if _, ok := vars[vd.Name]; !ok {
				if vd.Default != "" {
					vars[vd.Name] = vd.Default
				} else if vd.Required {
					return fmt.Errorf("required variable {{%s}} not provided (use --var %s=<value>)", vd.Name, vd.Name)
				}
			}
			// Validate enum constraint.
			if len(vd.Enum) > 0 {
				if val, ok := vars[vd.Name]; ok {
					valid := false
					for _, e := range vd.Enum {
						if val == e {
							valid = true
							break
						}
					}
					if !valid {
						return fmt.Errorf("variable {{%s}} value %q not in allowed values: %v", vd.Name, val, vd.Enum)
					}
				}
			}
		}

		// ── Step 2: Expand ──────────────────────────────────────
		type expandedStep struct {
			TemplateStep
			skip bool // condition evaluated to false
		}

		expanded := make([]expandedStep, 0, len(fields.Steps))
		for _, s := range fields.Steps {
			es := expandedStep{TemplateStep: s}

			// Evaluate condition.
			if s.Condition != "" {
				if !evaluateCondition(s.Condition, vars) {
					es.skip = true
				}
			}

			// Substitute variables in title and description.
			es.Title = substituteVars(s.Title, vars)
			es.Description = substituteVars(s.Description, vars)
			es.Assignee = substituteVars(s.Assignee, vars)

			expanded = append(expanded, es)
		}

		// Filter out skipped steps and update depends_on.
		skipped := make(map[string]bool)
		for _, es := range expanded {
			if es.skip {
				skipped[es.ID] = true
			}
		}

		var active []expandedStep
		for _, es := range expanded {
			if es.skip {
				continue
			}
			// Remove deps on skipped steps.
			var filteredDeps []string
			for _, dep := range es.DependsOn {
				if !skipped[dep] {
					filteredDeps = append(filteredDeps, dep)
				}
			}
			es.DependsOn = filteredDeps
			active = append(active, es)
		}

		if len(active) == 0 {
			return fmt.Errorf("all steps were filtered by conditions; nothing to create")
		}

		// ── Dry run ─────────────────────────────────────────────
		if dryRun {
			fmt.Printf("Template: %s (%s)\n", bead.Title, templateID)
			fmt.Printf("Variables: %v\n\n", vars)
			fmt.Println("Would create:")
			fmt.Printf("  Bundle: %s\n", substituteVars(bead.Title, vars))
			for _, s := range active {
				typ := s.Type
				if typ == "" {
					typ = "task"
				}
				deps := ""
				if len(s.DependsOn) > 0 {
					deps = fmt.Sprintf(" (after: %s)", strings.Join(s.DependsOn, ", "))
				}
				fmt.Printf("  Step %s: %s [%s]%s\n", s.ID, s.Title, typ, deps)
			}
			fmt.Printf("\nTotal: 1 bundle + %d steps\n", len(active))
			return nil
		}

		// ── Step 3: Create ──────────────────────────────────────

		// Create the root bundle bead.
		bundleTitle := substituteVars(bead.Title, vars)
		appliedVarsJSON, _ := json.Marshal(vars)
		bundleFields := map[string]any{
			"template_id":  templateID,
			"applied_vars": json.RawMessage(appliedVarsJSON),
		}
		bundleFieldsJSON, _ := json.Marshal(bundleFields)

		bundleReq := &client.CreateBeadRequest{
			Title:       bundleTitle,
			Description: substituteVars(bead.Description, vars),
			Type:        "bundle",
			Priority:    bead.Priority,
			Labels:      labels,
			Assignee:    assignee,
			CreatedBy:   actor,
			Fields:      bundleFieldsJSON,
		}

		bundle, err := beadsClient.CreateBead(ctx, bundleReq)
		if err != nil {
			return fmt.Errorf("creating bundle: %w", err)
		}

		// Create child beads for each step.
		stepBeadIDs := make(map[string]string, len(active)) // step ID → bead ID

		for _, s := range active {
			typ := s.Type
			if typ == "" {
				typ = "task"
			}
			stepAssignee := s.Assignee
			if stepAssignee == "" {
				stepAssignee = assignee
			}

			stepReq := &client.CreateBeadRequest{
				Title:       s.Title,
				Description: s.Description,
				Type:        typ,
				Priority:    priorityOrDefault(s.Priority, bead.Priority),
				Labels:      mergeLabels(labels, s.Labels),
				Assignee:    stepAssignee,
				CreatedBy:   actor,
			}

			stepBead, err := beadsClient.CreateBead(ctx, stepReq)
			if err != nil {
				return fmt.Errorf("creating step %q: %w", s.ID, err)
			}
			stepBeadIDs[s.ID] = stepBead.ID

			// Add parent-child dep to bundle.
			_, err = beadsClient.AddDependency(ctx, &client.AddDependencyRequest{
				BeadID:      stepBead.ID,
				DependsOnID: bundle.ID,
				Type:        "parent-child",
				CreatedBy:   actor,
			})
			if err != nil {
				return fmt.Errorf("linking step %q to bundle: %w", s.ID, err)
			}
		}

		// Add blocks dependencies between steps.
		for _, s := range active {
			for _, depStepID := range s.DependsOn {
				depBeadID, ok := stepBeadIDs[depStepID]
				if !ok {
					continue
				}
				_, err = beadsClient.AddDependency(ctx, &client.AddDependencyRequest{
					BeadID:      stepBeadIDs[s.ID],
					DependsOnID: depBeadID,
					Type:        "blocks",
					CreatedBy:   actor,
				})
				if err != nil {
					return fmt.Errorf("adding dependency %s→%s: %w", s.ID, depStepID, err)
				}
			}
		}

		// Output results.
		if jsonOutput {
			result := map[string]any{
				"bundle": bundle,
				"steps":  stepBeadIDs,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Created bundle %s: %s\n", bundle.ID, bundle.Title)
			fmt.Printf("  Template: %s\n", templateID)
			fmt.Printf("  Steps:    %d\n", len(active))
			for _, s := range active {
				fmt.Printf("    %s → %s: %s\n", s.ID, stepBeadIDs[s.ID], s.Title)
			}
		}
		return nil
	},
}

// varPattern matches {{variable}} placeholders.
var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// substituteVars replaces {{name}} placeholders with values from vars.
// Unmatched placeholders are left as-is.
func substituteVars(s string, vars map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-2]
		if val, ok := vars[name]; ok {
			return val
		}
		return match
	})
}

// evaluateCondition evaluates a simple condition string against variables.
// Supported formats:
//   - "{{var}}"          — truthy (non-empty)
//   - "!{{var}}"         — falsy (empty)
//   - "{{var}} == value" — equality
//   - "{{var}} != value" — inequality
func evaluateCondition(cond string, vars map[string]string) bool {
	cond = strings.TrimSpace(cond)

	// Negation: !{{var}}
	if strings.HasPrefix(cond, "!") {
		inner := strings.TrimPrefix(cond, "!")
		return !evaluateCondition(inner, vars)
	}

	// Equality/inequality: {{var}} == value / {{var}} != value
	for _, op := range []string{"!=", "=="} {
		if parts := strings.SplitN(cond, op, 2); len(parts) == 2 {
			left := substituteVars(strings.TrimSpace(parts[0]), vars)
			right := strings.TrimSpace(parts[1])
			if op == "==" {
				return left == right
			}
			return left != right
		}
	}

	// Simple truthy: {{var}} — evaluate to non-empty after substitution.
	resolved := substituteVars(cond, vars)
	// If the variable was substituted, check truthiness.
	if resolved != cond {
		return resolved != "" && resolved != "false" && resolved != "0"
	}
	// Unresolved placeholder — treat as falsy.
	if varPattern.MatchString(cond) {
		return false
	}
	// Literal non-empty string — truthy.
	return resolved != ""
}

// priorityOrDefault returns the step priority if set, otherwise the default.
func priorityOrDefault(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// mergeLabels combines two label slices, deduplicating.
func mergeLabels(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(extra))
	var result []string
	for _, l := range base {
		if !seen[l] {
			seen[l] = true
			result = append(result, l)
		}
	}
	for _, l := range extra {
		if !seen[l] {
			seen[l] = true
			result = append(result, l)
		}
	}
	return result
}

func init() {
	templateApplyCmd.Flags().StringSlice("var", nil, "variable substitution (key=value, repeatable)")
	templateApplyCmd.Flags().StringSliceP("label", "l", nil, "labels for created beads (repeatable)")
	templateApplyCmd.Flags().String("assignee", "", "default assignee for created beads")
	templateApplyCmd.Flags().Bool("dry-run", false, "preview what would be created without creating anything")
}

