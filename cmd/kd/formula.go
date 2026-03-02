package main

import "github.com/spf13/cobra"

var formulaCmd = &cobra.Command{
	Use:     "formula",
	Short:   "Manage reusable work formulas",
	Long:    "Formulas define reusable sets of work items (steps) with variable substitution.\nCreate a formula, then apply it to instantiate a molecule of beads.",
	GroupID: "beads",
}

// templateCmd is a hidden alias for backward compatibility.
var templateCmd = &cobra.Command{
	Use:     "template",
	Short:   "Manage reusable work templates (deprecated: use 'formula')",
	Long:    "Deprecated — use 'kd formula' instead.\n\nFormulas define reusable sets of work items (steps) with variable substitution.\nCreate a formula, then apply it to instantiate a molecule of beads.",
	GroupID: "beads",
	Hidden:  true,
}

func init() {
	for _, cmd := range []*cobra.Command{formulaCmd, templateCmd} {
		cmd.AddCommand(formulaCreateCmd)
		cmd.AddCommand(formulaListCmd)
		cmd.AddCommand(formulaShowCmd)
		cmd.AddCommand(formulaApplyCmd)
	}
}
