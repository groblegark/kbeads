package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/events"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var yieldCmd = &cobra.Command{
	Use:   "yield",
	Short: "Block until a pending decision is resolved or mail arrives",
	Long: `Blocks the agent until one of the following events occurs:
  - A pending decision bead (type=decision, status=open) is closed/resolved
  - A mail/message bead targeting this agent is created
  - The timeout expires (default 24h)

Uses NATS if BEADS_NATS_URL is set, otherwise polls every 2 seconds.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetDuration("timeout")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		// Find the most recent pending decision by this actor.
		resp, err := beadsClient.ListBeads(ctx, &client.ListBeadsRequest{
			Status: []string{"open"},
			Type:   []string{"decision"},
			Sort:   "-created_at",
			Limit:  1,
		})
		if err != nil {
			return fmt.Errorf("listing decisions: %w", err)
		}

		if len(resp.Beads) == 0 {
			fmt.Println("No pending decisions found, waiting for any event...")
		} else {
			d := resp.Beads[0]
			prompt := decisionField(d, "prompt")
			if prompt == "" {
				prompt = d.Title
			}
			fmt.Fprintf(os.Stderr, "Yielding on decision %s: %s\n", d.ID, prompt)
		}

		natsURL := os.Getenv("BEADS_NATS_URL")
		if natsURL == "" {
			natsURL = os.Getenv("COOP_NATS_URL")
		}

		if natsURL != "" {
			return yieldNATS(ctx, natsURL, resp.Beads)
		}
		return yieldPoll(ctx, resp.Beads)
	},
}

func yieldNATS(ctx context.Context, natsURL string, pending []*model.Bead) error {
	pendingIDs := make(map[string]bool, len(pending))
	for _, b := range pending {
		pendingIDs[b.ID] = true
	}

	sub, err := events.NewNATSSubscriber(natsURL,
		nats.ReconnectHandler(func(_ *nats.Conn) {}),
	)
	if err != nil {
		return yieldPoll(ctx, pending)
	}
	defer sub.Close()

	ch, cancel, err := sub.Subscribe("beads.>")
	if err != nil {
		return yieldPoll(ctx, pending)
	}
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Println("Yield timed out")
			}
			return nil
		case data, ok := <-ch:
			if !ok {
				return nil
			}
			var evt map[string]any
			if err := json.Unmarshal(data, &evt); err != nil {
				continue
			}
			beadID, _ := evt["bead_id"].(string)
			if pendingIDs[beadID] {
				return printYieldResult(beadID)
			}
			// Also wake on mail/message creation targeting us.
			beadType, _ := evt["type"].(string)
			if beadType == "message" || beadType == "mail" {
				fmt.Printf("Mail received: %s\n", beadID)
				return nil
			}
		}
	}
}

func yieldPoll(ctx context.Context, pending []*model.Bead) error {
	pendingIDs := make(map[string]bool, len(pending))
	for _, b := range pending {
		pendingIDs[b.ID] = true
	}

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Println("Yield timed out")
			}
			return nil
		case <-time.After(2 * time.Second):
		}

		// Check if any pending decision was resolved.
		for id := range pendingIDs {
			bead, err := beadsClient.GetBead(ctx, id)
			if err != nil {
				continue
			}
			if bead.Status == model.StatusClosed {
				return printYieldResult(id)
			}
			chosen := decisionField(bead, "chosen")
			responseText := decisionField(bead, "response_text")
			if chosen != "" || responseText != "" {
				return printYieldResult(id)
			}
		}

		// If no decisions, check for any new mail.
		if len(pendingIDs) == 0 {
			msgs, err := beadsClient.ListBeads(ctx, &client.ListBeadsRequest{
				Status: []string{"open"},
				Type:   []string{"message", "mail"},
				Limit:  1,
				Sort:   "-created_at",
			})
			if err == nil && len(msgs.Beads) > 0 {
				fmt.Printf("Mail received: %s\n", msgs.Beads[0].ID)
				return nil
			}
		}
	}
}

func printYieldResult(id string) error {
	bead, err := beadsClient.GetBead(context.Background(), id)
	if err != nil {
		return err
	}
	chosen := decisionField(bead, "chosen")
	responseText := decisionField(bead, "response_text")
	if chosen != "" {
		fmt.Printf("Decision %s resolved: %s\n", id, chosen)
	} else if responseText != "" {
		fmt.Printf("Decision %s resolved: %s\n", id, responseText)
	} else {
		fmt.Printf("Decision %s closed\n", id)
	}
	return nil
}

func init() {
	yieldCmd.Flags().Duration("timeout", 24*time.Hour, "maximum time to wait")
}
