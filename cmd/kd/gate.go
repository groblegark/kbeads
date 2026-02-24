package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Manage session gates",
}

var gateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show gate state for the current agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve agent bead ID: KD_AGENT_ID env > KD_ACTOR assignee lookup.
		agentID := os.Getenv("KD_AGENT_ID")
		if agentID == "" {
			actorName := os.Getenv("KD_ACTOR")
			if actorName == "" {
				actorName = actor
			}
			agentID = resolveAgentByActor(context.Background(), actorName, "")
		}
		if agentID == "" {
			fmt.Println("No agent identity found (set KD_AGENT_ID or KD_ACTOR)")
			return nil
		}

		gates, err := beadsClient.ListGates(cmd.Context(), agentID)
		if err != nil {
			return fmt.Errorf("listing gates: %w", err)
		}

		if len(gates) == 0 {
			fmt.Printf("No gates found for this agent.\n")
			return nil
		}

		fmt.Printf("Session gates for agent %s:\n", agentID)
		for _, g := range gates {
			var bullet string
			var detail string
			if g.Status == "satisfied" {
				bullet = "●"
				if g.SatisfiedAt != nil {
					detail = fmt.Sprintf(" (%s)", g.SatisfiedAt.Format("2006-01-02 15:04:05"))
				}
			} else {
				bullet = "○"
			}
			fmt.Printf("  %s %s: %s%s\n", bullet, g.GateID, g.Status, detail)
		}
		return nil
	},
}

var gateMarkCmd = &cobra.Command{
	Use:   "mark <gate-id>",
	Short: "Manually mark a gate as satisfied",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gateID := args[0]

		// Resolve agent bead ID.
		agentID := os.Getenv("KD_AGENT_ID")
		if agentID == "" {
			actorName := os.Getenv("KD_ACTOR")
			if actorName == "" {
				actorName = actor
			}
			agentID = resolveAgentByActor(cmd.Context(), actorName, "")
		}
		if agentID == "" {
			return fmt.Errorf("no agent identity found (set KD_AGENT_ID or KD_ACTOR)")
		}

		if err := beadsClient.SatisfyGate(cmd.Context(), agentID, gateID); err != nil {
			return fmt.Errorf("satisfying gate: %w", err)
		}

		fmt.Printf("✓ Gate %s marked as satisfied\n", gateID)
		return nil
	},
}

var gateClearCmd = &cobra.Command{
	Use:   "clear <gate-id>",
	Short: "Clear a gate (reset to pending)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		gateID := args[0]

		// Resolve agent bead ID.
		agentID := os.Getenv("KD_AGENT_ID")
		if agentID == "" {
			actorName := os.Getenv("KD_ACTOR")
			if actorName == "" {
				actorName = actor
			}
			agentID = resolveAgentByActor(cmd.Context(), actorName, "")
		}
		if agentID == "" {
			return fmt.Errorf("no agent identity found (set KD_AGENT_ID or KD_ACTOR)")
		}

		if err := beadsClient.ClearGate(cmd.Context(), agentID, gateID); err != nil {
			return fmt.Errorf("clearing gate: %w", err)
		}

		fmt.Printf("○ Gate %s cleared (pending)\n", gateID)
		return nil
	},
}

func init() {
	gateCmd.AddCommand(gateStatusCmd)
	gateCmd.AddCommand(gateMarkCmd)
	gateCmd.AddCommand(gateClearCmd)
}
