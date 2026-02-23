package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

var jackDownCmd = &cobra.Command{
	Use:   "down <jack-id>",
	Short: "Close a jack (revert and close change permit)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		reason, _ := cmd.Flags().GetString("reason")
		skipRevert, _ := cmd.Flags().GetBool("skip-revert-check")

		if reason == "" {
			fmt.Fprintln(os.Stderr, "Error: --reason is required")
			os.Exit(1)
		}
		if skipRevert && len(reason) < 10 {
			fmt.Fprintln(os.Stderr, "Error: --skip-revert-check requires a reason of at least 10 characters")
			os.Exit(1)
		}

		// Fetch current bead to get existing fields.
		bead, err := beadsClient.GetBead(context.Background(), id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if string(bead.Type) != "jack" {
			fmt.Fprintf(os.Stderr, "Error: %s is not a jack (type=%s)\n", id, bead.Type)
			os.Exit(1)
		}

		// Merge new fields into existing fields.
		var fields map[string]any
		if len(bead.Fields) > 0 {
			json.Unmarshal(bead.Fields, &fields)
		}
		if fields == nil {
			fields = make(map[string]any)
		}
		fields["jack_reverted"] = !skipRevert
		fields["jack_closed_reason"] = reason
		fields["jack_closed_at"] = time.Now().UTC().Format(time.RFC3339)

		fieldsJSON, _ := json.Marshal(fields)

		// Close the bead.
		_, err = beadsClient.UpdateBead(context.Background(), id, &client.UpdateBeadRequest{
			Fields: fieldsJSON,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating fields: %v\n", err)
			os.Exit(1)
		}

		_, err = beadsClient.CloseBead(context.Background(), id, actor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error closing jack: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			bead, _ = beadsClient.GetBead(context.Background(), id)
			printBeadJSON(bead)
		} else {
			fmt.Printf("Jack closed: %s\n", id)
			fmt.Printf("  Reason: %s\n", reason)
			if skipRevert {
				fmt.Println("  Revert check: skipped")
			} else {
				fmt.Println("  Reverted: yes")
			}
		}
		return nil
	},
}

func init() {
	jackDownCmd.Flags().StringP("reason", "r", "", "reason for closing (required)")
	jackDownCmd.Flags().Bool("skip-revert-check", false, "close without verifying revert")
}
