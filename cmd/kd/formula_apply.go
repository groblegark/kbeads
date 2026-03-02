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

var formulaApplyCmd = &cobra.Command{
	Use:   "apply <formula-id>",
	Short: "Apply a formula to create a molecule of beads",
	Long: `Apply a formula to create a molecule (an epic with child beads).

The 3-step pipeline:
  1. Resolve — fetch the formula and parse its vars/steps
  2. Expand  — substitute {{variables}} with provided values
  3. Create  — create the molecule bead and child issue beads

Examples:
  kd formula apply kd-abc123 --var component=auth --var assignee=alice
  kd formula apply kd-abc123 --var component=auth --dry-run
  kd formula apply kd-abc123 --var component=auth --label project:gasboat`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		formulaID := args[0]
		varPairs, _ := cmd.Flags().GetStringSlice("var")
		labels, _ := cmd.Flags().GetStringSlice("label")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		assignee, _ := cmd.Flags().GetString("assignee")

		ctx := context.Background()

		// ── Step 1: Resolve ─────────────────────────────────────
		bead, err := beadsClient.GetBead(ctx, formulaID)
		if err != nil {
			return fmt.Errorf("resolving formula: %w", err)
		}
		// Accept both "formula" and legacy "template" type.
		if string(bead.Type) != "formula" && string(bead.Type) != "template" {
			return fmt.Errorf("bead %s is type %q, not formula", formulaID, bead.Type)
		}

		var fields struct {
			Vars  []FormulaVarDef `json:"vars"`
			Steps []FormulaStep   `json:"steps"`
		}
		if len(bead.Fields) == 0 {
			return fmt.Errorf("formula %s has no fields (empty formula)", formulaID)
		}
		if err := json.Unmarshal(bead.Fields, &fields); err != nil {
			return fmt.Errorf("parsing formula fields: %w", err)
		}
		if len(fields.Steps) == 0 {
			return fmt.Errorf("formula %s has no steps", formulaID)
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
			FormulaStep
			skip bool // condition evaluated to false
		}

		expanded := make([]expandedStep, 0, len(fields.Steps))
		for _, s := range fields.Steps {
			es := expandedStep{FormulaStep: s}

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
			fmt.Printf("Formula: %s (%s)\n", bead.Title, formulaID)
			fmt.Printf("Variables: %v\n\n", vars)
			fmt.Println("Would create:")
			fmt.Printf("  Molecule: %s\n", substituteVars(bead.Title, vars))
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
			fmt.Printf("\nTotal: 1 molecule + %d steps\n", len(active))
			return nil
		}

		// ── Step 3: Create ──────────────────────────────────────

		// Create the root molecule bead.
		molTitle := substituteVars(bead.Title, vars)
		appliedVarsJSON, _ := json.Marshal(vars)
		molFields := map[string]any{
			"formula_id":  formulaID,
			"applied_vars": json.RawMessage(appliedVarsJSON),
		}
		molFieldsJSON, _ := json.Marshal(molFields)

		molReq := &client.CreateBeadRequest{
			Title:       molTitle,
			Description: substituteVars(bead.Description, vars),
			Type:        "molecule",
			Priority:    bead.Priority,
			Labels:      labels,
			Assignee:    assignee,
			CreatedBy:   actor,
			Fields:      molFieldsJSON,
		}

		mol, err := beadsClient.CreateBead(ctx, molReq)
		if err != nil {
			return fmt.Errorf("creating molecule: %w", err)
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

			// Add parent-child dep to molecule.
			_, err = beadsClient.AddDependency(ctx, &client.AddDependencyRequest{
				BeadID:      stepBead.ID,
				DependsOnID: mol.ID,
				Type:        "parent-child",
				CreatedBy:   actor,
			})
			if err != nil {
				return fmt.Errorf("linking step %q to molecule: %w", s.ID, err)
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
				"molecule": mol,
				"steps":    stepBeadIDs,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("Created molecule %s: %s\n", mol.ID, mol.Title)
			fmt.Printf("  Formula: %s\n", formulaID)
			fmt.Printf("  Steps:   %d\n", len(active))
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
	formulaApplyCmd.Flags().StringSlice("var", nil, "variable substitution (key=value, repeatable)")
	formulaApplyCmd.Flags().StringSliceP("label", "l", nil, "labels for created beads (repeatable)")
	formulaApplyCmd.Flags().String("assignee", "", "default assignee for created beads")
	formulaApplyCmd.Flags().Bool("dry-run", false, "preview what would be created without creating anything")
}
