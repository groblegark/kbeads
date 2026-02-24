package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/groblegark/kbeads/internal/events"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var decisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Manage decision points",
}

// ── decision create ─────────────────────────────────────────────────────

var decisionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a decision point and optionally wait for response",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		optionsJSON, _ := cmd.Flags().GetString("options")
		requestedBy, _ := cmd.Flags().GetString("requested-by")
		decisionCtx, _ := cmd.Flags().GetString("context")
		noWait, _ := cmd.Flags().GetBool("no-wait")

		if prompt == "" {
			return fmt.Errorf("--prompt is required")
		}

		// Build fields for the decision bead.
		fields := map[string]any{
			"prompt": prompt,
		}
		if optionsJSON != "" {
			// Validate options JSON.
			var opts []any
			if err := json.Unmarshal([]byte(optionsJSON), &opts); err != nil {
				return fmt.Errorf("invalid --options JSON: %w", err)
			}
			fields["options"] = json.RawMessage(optionsJSON)
		}
		if decisionCtx != "" {
			fields["context"] = decisionCtx
		}
		if requestedBy == "" {
			requestedBy = actor
		}
		fields["requested_by"] = requestedBy

		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("encoding fields: %w", err)
		}

		req := &client.CreateBeadRequest{
			Title:     prompt,
			Type:      "decision",
			Kind:      "data",
			Priority:  2,
			CreatedBy: actor,
			Fields:    fieldsJSON,
		}

		bead, err := beadsClient.CreateBead(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			fmt.Printf("Created decision: %s\n", bead.ID)
			printDecisionSummary(bead)
		}

		if noWait {
			return nil
		}

		// Wait for resolution (bead closed with chosen field set).
		fmt.Fprintf(os.Stderr, "Waiting for response...\n")
		return waitForDecision(bead.ID)
	},
}

// ── decision list ─────────────────────────────────────────────────────

var decisionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List decision points",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetStringSlice("status")
		limit, _ := cmd.Flags().GetInt("limit")

		if len(status) == 0 {
			status = []string{"open", "in_progress"}
		}

		req := &client.ListBeadsRequest{
			Status: status,
			Type:   []string{"decision"},
			Limit:  limit,
			Sort:   "-created_at",
		}

		resp, err := beadsClient.ListBeads(context.Background(), req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadListJSON(resp.Beads)
		} else if len(resp.Beads) == 0 {
			fmt.Println("No pending decisions")
		} else {
			for _, b := range resp.Beads {
				printDecisionSummary(b)
				fmt.Println()
			}
		}
		return nil
	},
}

// ── decision show ─────────────────────────────────────────────────────

var decisionShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of a decision point",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bead, err := beadsClient.GetBead(context.Background(), args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			printDecisionDetail(bead)
		}
		return nil
	},
}

// ── decision respond ──────────────────────────────────────────────────

var decisionRespondCmd = &cobra.Command{
	Use:   "respond <id>",
	Short: "Respond to a decision point",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		selected, _ := cmd.Flags().GetString("select")
		text, _ := cmd.Flags().GetString("text")

		if selected == "" && text == "" {
			return fmt.Errorf("--select or --text is required")
		}

		// Update the fields with chosen response, then close.
		fields := map[string]any{}
		if selected != "" {
			fields["chosen"] = selected
		}
		if text != "" {
			fields["response_text"] = text
		}
		fields["responded_by"] = actor
		fields["responded_at"] = time.Now().UTC().Format(time.RFC3339)

		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("encoding fields: %w", err)
		}

		// Merge fields via update.
		_, err = beadsClient.UpdateBead(context.Background(), id, &client.UpdateBeadRequest{
			Fields: fieldsJSON,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error updating decision: %v\n", err)
			os.Exit(1)
		}

		// Close the decision bead.
		bead, err := beadsClient.CloseBead(context.Background(), id, actor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error closing decision: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			printBeadJSON(bead)
		} else {
			fmt.Printf("Decision %s resolved\n", id)
		}
		return nil
	},
}

// ── helpers ────────────────────────────────────────────────────────────

func printDecisionSummary(b *model.Bead) {
	prompt := decisionField(b, "prompt")
	if prompt == "" {
		prompt = b.Title
	}
	status := string(b.Status)
	chosen := decisionField(b, "chosen")
	if chosen != "" {
		status = "resolved: " + chosen
	}

	fmt.Printf("  %s [%s] %s\n", b.ID, status, prompt)

	// Print options if available.
	optionsRaw := decisionField(b, "options")
	if optionsRaw != "" {
		var opts []map[string]any
		if err := json.Unmarshal([]byte(optionsRaw), &opts); err == nil {
			for _, opt := range opts {
				id, _ := opt["id"].(string)
				label, _ := opt["label"].(string)
				if label == "" {
					label, _ = opt["short"].(string)
				}
				fmt.Printf("    [%s] %s\n", id, label)
			}
		}
	}
}

func printDecisionDetail(b *model.Bead) {
	fmt.Printf("ID:       %s\n", b.ID)
	fmt.Printf("Status:   %s\n", b.Status)

	prompt := decisionField(b, "prompt")
	if prompt != "" {
		fmt.Printf("Prompt:   %s\n", prompt)
	} else {
		fmt.Printf("Title:    %s\n", b.Title)
	}

	ctx := decisionField(b, "context")
	if ctx != "" {
		fmt.Printf("Context:  %s\n", ctx)
	}

	optionsRaw := decisionField(b, "options")
	if optionsRaw != "" {
		fmt.Println("Options:")
		var opts []map[string]any
		if err := json.Unmarshal([]byte(optionsRaw), &opts); err == nil {
			for _, opt := range opts {
				id, _ := opt["id"].(string)
				label, _ := opt["label"].(string)
				short, _ := opt["short"].(string)
				if label != "" {
					fmt.Printf("  [%s] %s — %s\n", id, short, label)
				} else {
					fmt.Printf("  [%s] %s\n", id, short)
				}
			}
		}
	}

	chosen := decisionField(b, "chosen")
	if chosen != "" {
		fmt.Printf("Chosen:   %s\n", chosen)
	}
	respText := decisionField(b, "response_text")
	if respText != "" {
		fmt.Printf("Response: %s\n", respText)
	}
	respondedBy := decisionField(b, "responded_by")
	if respondedBy != "" {
		fmt.Printf("By:       %s\n", respondedBy)
	}

	if !b.CreatedAt.IsZero() {
		fmt.Printf("Created:  %s\n", b.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func decisionField(b *model.Bead, key string) string {
	if len(b.Fields) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b.Fields, &m); err != nil {
		return ""
	}
	raw, ok := m[key]
	if !ok {
		return ""
	}
	// Try to unquote a JSON string; fall back to raw representation.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

// waitForDecision blocks until the decision bead is closed or the context
// is cancelled. Uses NATS if available, otherwise polls.
func waitForDecision(id string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	natsURL := os.Getenv("BEADS_NATS_URL")
	if natsURL == "" {
		natsURL = os.Getenv("COOP_NATS_URL")
	}
	if natsURL != "" {
		return waitDecisionNATS(ctx, natsURL, id)
	}
	return waitDecisionPoll(ctx, id)
}

func waitDecisionNATS(ctx context.Context, natsURL, id string) error {
	sub, err := events.NewNATSSubscriber(natsURL,
		nats.ReconnectHandler(func(_ *nats.Conn) {}),
	)
	if err != nil {
		// Fall back to polling on NATS failure.
		return waitDecisionPoll(ctx, id)
	}
	defer sub.Close()

	ch, cancel, err := sub.Subscribe("beads.>")
	if err != nil {
		return waitDecisionPoll(ctx, id)
	}
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case data, ok := <-ch:
			if !ok {
				return nil
			}
			// Check if this event is for our decision bead.
			var evt map[string]any
			if err := json.Unmarshal(data, &evt); err != nil {
				continue
			}
			beadID, _ := evt["bead_id"].(string)
			if beadID != id {
				continue
			}
			return printDecisionResult(id)
		case <-time.After(30 * time.Minute):
			return fmt.Errorf("timeout waiting for decision %s", id)
		}
	}
}

func waitDecisionPoll(ctx context.Context, id string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}

		bead, err := beadsClient.GetBead(ctx, id)
		if err != nil {
			continue
		}
		if bead.Status == model.StatusClosed {
			return printDecisionResult(id)
		}
		// Also check if chosen field is set (resolution without close).
		chosen := decisionField(bead, "chosen")
		responseText := decisionField(bead, "response_text")
		if chosen != "" || responseText != "" {
			printDecisionDetail(bead)
			return nil
		}
	}
}

func printDecisionResult(id string) error {
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
	decisionCmd.AddCommand(decisionCreateCmd)
	decisionCmd.AddCommand(decisionListCmd)
	decisionCmd.AddCommand(decisionShowCmd)
	decisionCmd.AddCommand(decisionRespondCmd)

	// create flags
	decisionCreateCmd.Flags().String("prompt", "", "decision prompt (required)")
	decisionCreateCmd.Flags().String("options", "", "options JSON array")
	decisionCreateCmd.Flags().String("requested-by", "", "who is requesting (default: actor)")
	decisionCreateCmd.Flags().String("context", "", "background context for the decision")
	decisionCreateCmd.Flags().Bool("no-wait", false, "return immediately without waiting for response")

	// list flags
	decisionListCmd.Flags().StringSliceP("status", "s", nil, "filter by status")
	decisionListCmd.Flags().Int("limit", 20, "maximum number of results")

	// respond flags
	decisionRespondCmd.Flags().String("select", "", "selected option ID")
	decisionRespondCmd.Flags().String("text", "", "free-text response")
}
