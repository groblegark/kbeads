package main

import "github.com/spf13/cobra"

var templateCmd = &cobra.Command{
	Use:     "template",
	Short:   "Manage reusable work templates",
	Long:    "Templates define reusable sets of work items (steps) with variable substitution.\nCreate a template, then apply it to instantiate a bundle of beads.",
	GroupID: "beads",
}

func init() {
	templateCmd.AddCommand(templateCreateCmd)
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateApplyCmd)
}
