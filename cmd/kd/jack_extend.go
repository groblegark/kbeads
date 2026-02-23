package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/spf13/cobra"
)

var jackExtendCmd = &cobra.Command{
	Use:   "extend <jack-id>",
	Short: "Extend a jack's TTL",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		ttlStr, _ := cmd.Flags().GetString("ttl")
		reason, _ := cmd.Flags().GetString("reason")

		if ttlStr == "" {
			fmt.Fprintln(os.Stderr, "Error: --ttl is required")
			os.Exit(1)
		}

		ttl, err := time.ParseDuration(ttlStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid TTL %q: %v\n", ttlStr, err)
			os.Exit(1)
		}
		if ttl > model.JackMaxSingleExtension {
			fmt.Fprintf(os.Stderr, "Error: single extension %v exceeds maximum %v\n", ttl, model.JackMaxSingleExtension)
			os.Exit(1)
		}

		// Fetch current jack.
		bead, err := beadsClient.GetBead(context.Background(), id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if string(bead.Type) != "jack" {
			fmt.Fprintf(os.Stderr, "Error: %s is not a jack\n", id)
			os.Exit(1)
		}
		if string(bead.Status) == "closed" {
			fmt.Fprintf(os.Stderr, "Error: jack %s is already closed\n", id)
			os.Exit(1)
		}

		var fields map[string]any
		if len(bead.Fields) > 0 {
			json.Unmarshal(bead.Fields, &fields)
		}
		if fields == nil {
			fields = make(map[string]any)
		}

		// Check extension count.
		extCount := 0
		if v, ok := fields["jack_extension_count"].(float64); ok {
			extCount = int(v)
		}
		if extCount >= model.JackMaxExtensions {
			fmt.Fprintf(os.Stderr, "Error: jack has reached maximum %d extensions\n", model.JackMaxExtensions)
			os.Exit(1)
		}

		// Check cumulative TTL.
		var cumulativeTTL time.Duration
		if v, ok := fields["jack_cumulative_ttl"].(string); ok && v != "" {
			cumulativeTTL, _ = time.ParseDuration(v)
		}
		if cumulativeTTL+ttl > model.JackMaxCumulativeTTL {
			fmt.Fprintf(os.Stderr, "Error: cumulative TTL %v + %v exceeds maximum %v\n",
				cumulativeTTL, ttl, model.JackMaxCumulativeTTL)
			os.Exit(1)
		}

		// Save original TTL on first extension.
		if extCount == 0 {
			if v, ok := fields["jack_ttl"]; ok {
				fields["jack_original_ttl"] = v
			}
		}

		now := time.Now().UTC()
		newExpiry := now.Add(ttl)
		fields["jack_expires_at"] = newExpiry.Format(time.RFC3339)
		fields["jack_extension_count"] = extCount + 1
		fields["jack_cumulative_ttl"] = (cumulativeTTL + ttl).String()

		fieldsJSON, _ := json.Marshal(fields)

		_, err = beadsClient.UpdateBead(context.Background(), id, &client.UpdateBeadRequest{
			Fields: fieldsJSON,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Add comment recording extension.
		comment := fmt.Sprintf("Extended TTL by %s (extension %d/%d)", ttlStr, extCount+1, model.JackMaxExtensions)
		if reason != "" {
			comment += ": " + reason
		}
		beadsClient.AddComment(context.Background(), id, actor, comment)

		if jsonOutput {
			bead, _ = beadsClient.GetBead(context.Background(), id)
			printBeadJSON(bead)
		} else {
			fmt.Printf("Jack extended: %s\n", id)
			fmt.Printf("  New expiry: %s\n", newExpiry.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Extensions: %d/%d\n", extCount+1, model.JackMaxExtensions)
		}
		return nil
	},
}

func init() {
	jackExtendCmd.Flags().String("ttl", "", "additional TTL duration (required)")
	jackExtendCmd.Flags().StringP("reason", "r", "", "why more time is needed")
}
