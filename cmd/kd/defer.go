package main

import (
	"context"
	"fmt"
	"os"
	"time"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var deferCmd = &cobra.Command{
	Use:   "defer <id>...",
	Short: "Defer one or more beads",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		until, _ := cmd.Flags().GetString("until")

		var deferUntil *timestamppb.Timestamp
		if until != "" {
			t, err := time.Parse(time.RFC3339, until)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing --until: %v\n", err)
				os.Exit(1)
			}
			deferUntil = timestamppb.New(t)
		}

		statusVal := "deferred"
		for _, id := range args {
			req := &beadsv1.UpdateBeadRequest{
				Id:         id,
				Status:     &statusVal,
				DeferUntil: deferUntil,
			}
			resp, err := client.UpdateBead(ctx, req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error deferring %s: %v\n", id, err)
				os.Exit(1)
			}
			if jsonOutput {
				printBeadJSON(resp.GetBead())
			} else {
				printBeadTable(resp.GetBead())
				if len(args) > 1 {
					fmt.Println()
				}
			}
		}
		return nil
	},
}

var undeferCmd = &cobra.Command{
	Use:   "undefer <id>...",
	Short: "Undefer one or more beads (set status to open, clear defer_until)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		statusVal := "open"
		clearDefer := timestamppb.New(time.Time{})

		for _, id := range args {
			req := &beadsv1.UpdateBeadRequest{
				Id:         id,
				Status:     &statusVal,
				DeferUntil: clearDefer,
			}
			resp, err := client.UpdateBead(ctx, req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error undeferring %s: %v\n", id, err)
				os.Exit(1)
			}
			if jsonOutput {
				printBeadJSON(resp.GetBead())
			} else {
				printBeadTable(resp.GetBead())
				if len(args) > 1 {
					fmt.Println()
				}
			}
		}
		return nil
	},
}

func init() {
	deferCmd.Flags().String("until", "", "defer until date (RFC3339 format, e.g. 2025-03-01T00:00:00Z)")
}
