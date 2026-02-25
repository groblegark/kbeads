package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/groblegark/kbeads/internal/hooks"
	"github.com/groblegark/kbeads/internal/presence"
)

// hookEmitRequest is the JSON body for POST /v1/hooks/emit.
type hookEmitRequest struct {
	AgentBeadID     string `json:"agent_bead_id"`
	HookType        string `json:"hook_type"`        // "Stop", "PreToolUse", etc.
	ClaudeSessionID string `json:"claude_session_id"`
	CWD             string `json:"cwd"`
	Actor           string `json:"actor"`
	ToolName        string `json:"tool_name,omitempty"` // tool name from Claude Code (e.g. "Bash", "Read")
}

// hookEmitResponse is the JSON response from POST /v1/hooks/emit.
type hookEmitResponse struct {
	Block    bool     `json:"block,omitempty"`
	Reason   string   `json:"reason,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Inject   string   `json:"inject,omitempty"`
}

// handleHookEmit handles POST /v1/hooks/emit.
// It evaluates gate state and soft auto-checks, returning block/warn/inject signals.
func (s *BeadsServer) handleHookEmit(w http.ResponseWriter, r *http.Request) {
	var req hookEmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Record presence for agent roster tracking.
	if s.Presence != nil && req.Actor != "" {
		s.Presence.RecordHookEvent(presence.HookEvent{
			Actor:     req.Actor,
			HookType:  req.HookType,
			ToolName:  req.ToolName,
			SessionID: req.ClaudeSessionID,
			CWD:       req.CWD,
		})
	}

	ctx := r.Context()
	var resp hookEmitResponse

	// If no agent_bead_id, there are no gates to check — allow.
	if req.AgentBeadID == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Evaluate strict gates for Stop hook.
	if req.HookType == "Stop" {
		// Upsert the decision gate for this agent (creates pending row if not exists).
		if err := s.store.UpsertGate(ctx, req.AgentBeadID, "decision"); err != nil {
			slog.Warn("hookEmit: failed to upsert decision gate", "agent", req.AgentBeadID, "err", err)
		}

		satisfied, err := s.store.IsGateSatisfied(ctx, req.AgentBeadID, "decision")
		if err != nil {
			slog.Warn("hookEmit: failed to check decision gate", "agent", req.AgentBeadID, "err", err)
		}
		if !satisfied {
			resp.Block = true
			resp.Reason = "decision: decision point offered before session end"
			writeJSON(w, http.StatusOK, resp)
			return
		}
	}

	// Soft gate AutoChecks — always warn, never block.
	if warning := hooks.CheckCommitPush(req.CWD); warning != "" {
		resp.Warnings = append(resp.Warnings, warning)
	}

	// TODO: bead-update soft check — requires checking KD_HOOK_BEAD env var from the
	// client side. Skip server-side check for now; the CLI can handle this in future.

	// Run existing advice hook logic for session-end trigger on Stop hook.
	if req.HookType == "Stop" {
		agentID := req.AgentBeadID
		if req.Actor != "" {
			agentID = req.Actor
		}
		hookResp := s.hooksHandler.HandleSessionEvent(ctx, hooks.SessionEvent{
			AgentID: agentID,
			Trigger: hooks.TriggerSessionEnd,
			CWD:     req.CWD,
		})
		if hookResp.Block && !resp.Block {
			resp.Block = true
			resp.Reason = hookResp.Reason
		}
		resp.Warnings = append(resp.Warnings, hookResp.Warnings...)
	}

	writeJSON(w, http.StatusOK, resp)
}

// executeHooksRequest is the JSON body for POST /v1/hooks/execute.
type executeHooksRequest struct {
	AgentID string `json:"agent_id"`
	Trigger string `json:"trigger"`
	CWD     string `json:"cwd,omitempty"`
}

// handleExecuteHooks handles POST /v1/hooks/execute.
// Agents call this to evaluate advice hooks for a given lifecycle trigger.
func (s *BeadsServer) handleExecuteHooks(w http.ResponseWriter, r *http.Request) {
	var req executeHooksRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if req.Trigger == "" {
		writeError(w, http.StatusBadRequest, "trigger is required")
		return
	}

	resp := s.hooksHandler.HandleSessionEvent(r.Context(), hooks.SessionEvent{
		AgentID: req.AgentID,
		Trigger: req.Trigger,
		CWD:     req.CWD,
	})

	writeJSON(w, http.StatusOK, resp)
}
