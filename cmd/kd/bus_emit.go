package main

// kd bus emit --hook=Stop
// Reads Claude Code hook event JSON from stdin, resolves agent identity,
// calls POST /v1/hooks/emit, and exits with appropriate code.
//
// Exit codes:
//
//	0 — allow
//	2 — block (stderr: {"decision":"block","reason":"..."})

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/groblegark/kbeads/internal/client"
	"github.com/spf13/cobra"
)

// busCmd is the parent command for event bus operations.
var busCmd = &cobra.Command{
	Use:   "bus",
	Short: "Event bus operations",
}

// busEmitCmd emits a hook event to the server.
var busEmitCmd = &cobra.Command{
	Use:   "emit",
	Short: "Emit a hook event",
	Long: `Reads a Claude Code hook event JSON from stdin, resolves agent identity,
and calls POST /v1/hooks/emit on the kbeads server.

Exit codes:
  0 — allow (or no gates to check)
  2 — block

Warnings are written to stdout as <system-reminder> tags for Claude Code.
Block reason is written to stderr as {"decision":"block","reason":"..."}.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hookType, _ := cmd.Flags().GetString("hook")
		if hookType == "" {
			return fmt.Errorf("--hook is required (e.g. Stop, PreToolUse, UserPromptSubmit, PreCompact)")
		}

		cwdFlag, _ := cmd.Flags().GetString("cwd")

		// Read JSON from stdin (Claude Code hook event format).
		var stdinEvent map[string]any
		decoder := json.NewDecoder(os.Stdin)
		if err := decoder.Decode(&stdinEvent); err != nil {
			// stdin may be empty or non-JSON (e.g. called manually).
			// Treat as empty event — proceed with env-based resolution.
			stdinEvent = map[string]any{}
		}

		// Resolve CWD: flag > stdin cwd field > os.Getwd().
		cwd := cwdFlag
		if cwd == "" {
			if v, ok := stdinEvent["cwd"].(string); ok && v != "" {
				cwd = v
			}
		}
		if cwd == "" {
			if wd, err := os.Getwd(); err == nil {
				cwd = wd
			}
		}

		// Extract claude_session_id from stdin JSON.
		claudeSessionID, _ := stdinEvent["session_id"].(string)

		// Resolve agent_bead_id in priority order:
		//   1. KD_AGENT_ID env var
		//   2. Query by KD_ACTOR name (assignee search)
		//   3. Empty string (no gates to check)
		agentBeadID := os.Getenv("KD_AGENT_ID")
		if agentBeadID == "" {
			agentBeadID = resolveAgentByActor(cmd.Context(), actor, claudeSessionID)
		}

		req := &client.EmitHookRequest{
			AgentBeadID:     agentBeadID,
			HookType:        hookType,
			ClaudeSessionID: claudeSessionID,
			CWD:             cwd,
			Actor:           actor,
		}

		resp, err := beadsClient.EmitHook(cmd.Context(), req)
		if err != nil {
			// On server error, allow (fail open) — don't block the agent.
			fmt.Fprintf(os.Stderr, "kd bus emit: server error (failing open): %v\n", err)
			os.Exit(0)
		}

		// Write warnings as system-reminder tags to stdout (Claude Code reads these).
		for _, w := range resp.Warnings {
			fmt.Printf("<system-reminder>%s</system-reminder>\n", w)
		}

		// Write inject content to stdout if present.
		if resp.Inject != "" {
			fmt.Print(resp.Inject)
		}

		// Block: write to stderr and exit 2.
		if resp.Block {
			blockJSON, _ := json.Marshal(map[string]string{
				"decision": "block",
				"reason":   resp.Reason,
			})
			fmt.Fprintf(os.Stderr, "%s\n", blockJSON)
			os.Exit(2)
		}

		return nil
	},
}

// resolveAgentByActor looks up an open agent bead by the actor's assignee name.
// Returns empty string if not found or on error.
func resolveAgentByActor(ctx context.Context, actorName, _ string) string {
	if actorName == "" || actorName == "unknown" {
		return ""
	}

	resp, err := beadsClient.ListBeads(ctx, &client.ListBeadsRequest{
		Type:     []string{"agent"},
		Assignee: actorName,
		Status:   []string{"open", "in_progress"},
		Sort:     "-created_at",
		Limit:    1,
	})
	if err != nil {
		return ""
	}
	if len(resp.Beads) == 0 {
		return ""
	}
	return resp.Beads[0].ID
}

func init() {
	busCmd.AddCommand(busEmitCmd)

	busEmitCmd.Flags().String("hook", "", "hook type: Stop|PreToolUse|UserPromptSubmit|PreCompact (required)")
	busEmitCmd.Flags().String("cwd", "", "working directory (default: current dir)")

	// Mark --hook as required so cobra prints a helpful error if omitted.
	_ = busEmitCmd.MarkFlagRequired("hook")
}

// printSystemReminder writes text as a Claude Code system-reminder tag to stdout.
// Exported so other commands can reuse it if needed.
func printSystemReminder(text string) {
	fmt.Printf("<system-reminder>%s</system-reminder>\n", strings.TrimSpace(text))
}
