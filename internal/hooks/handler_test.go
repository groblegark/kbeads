package hooks

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/groblegark/kbeads/internal/model"
	"github.com/groblegark/kbeads/internal/store"
)

// mockStore implements store.Store with only the methods needed for testing.
type mockStore struct {
	store.Store // embed to satisfy the full interface
	beads       []*model.Bead
}

func (m *mockStore) ListBeads(_ context.Context, _ model.BeadFilter) ([]*model.Bead, int, error) {
	return m.beads, len(m.beads), nil
}

func makeAdvice(id, title string, labels []string, fields map[string]any) *model.Bead {
	b, _ := json.Marshal(fields)
	return &model.Bead{
		ID:     id,
		Kind:   model.KindData,
		Type:   model.TypeAdvice,
		Title:  title,
		Status: model.StatusOpen,
		Labels: labels,
		Fields: b,
	}
}

func TestHandleSessionEvent_NoAgent(t *testing.T) {
	h := NewHandler(&mockStore{}, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	resp := h.HandleSessionEvent(context.Background(), SessionEvent{})
	if resp.Block {
		t.Error("expected no block for empty event")
	}
}

func TestHandleSessionEvent_NoMatchingAdvice(t *testing.T) {
	s := &mockStore{beads: []*model.Bead{
		makeAdvice("kd-1", "unrelated", []string{"rig:other"}, map[string]any{
			"hook_command": "echo hello",
			"hook_trigger": TriggerSessionEnd,
		}),
	}}
	h := NewHandler(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp := h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "beads/crew/test-agent",
		Trigger: TriggerSessionEnd,
	})
	if resp.Block {
		t.Error("expected no block when no advice matches")
	}
}

func TestHandleSessionEvent_MatchingHookRuns(t *testing.T) {
	s := &mockStore{beads: []*model.Bead{
		makeAdvice("kd-2", "global advice", []string{"global"}, map[string]any{
			"hook_command":    "echo ran",
			"hook_trigger":    TriggerSessionEnd,
			"hook_on_failure": OnFailureIgnore,
		}),
	}}
	h := NewHandler(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp := h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "beads/crew/test-agent",
		Trigger: TriggerSessionEnd,
	})
	if resp.Block {
		t.Error("expected no block for successful hook")
	}
}

func TestHandleSessionEvent_BlockOnFailure(t *testing.T) {
	s := &mockStore{beads: []*model.Bead{
		makeAdvice("kd-3", "blocking advice", []string{"global"}, map[string]any{
			"hook_command":    "exit 1",
			"hook_trigger":    TriggerSessionEnd,
			"hook_on_failure": OnFailureBlock,
		}),
	}}
	h := NewHandler(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp := h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "beads/crew/test-agent",
		Trigger: TriggerSessionEnd,
	})
	if !resp.Block {
		t.Error("expected block when hook fails with on_failure=block")
	}
	if resp.Reason == "" {
		t.Error("expected non-empty reason for block")
	}
}

func TestHandleSessionEvent_WarnOnFailure(t *testing.T) {
	s := &mockStore{beads: []*model.Bead{
		makeAdvice("kd-4", "warning advice", []string{"global"}, map[string]any{
			"hook_command":    "exit 1",
			"hook_trigger":    TriggerSessionEnd,
			"hook_on_failure": OnFailureWarn,
		}),
	}}
	h := NewHandler(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp := h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "beads/crew/test-agent",
		Trigger: TriggerSessionEnd,
	})
	if resp.Block {
		t.Error("expected no block for warn mode")
	}
	if len(resp.Warnings) == 0 {
		t.Error("expected warning message")
	}
}

func TestHandleSessionEvent_WrongTriggerSkipped(t *testing.T) {
	s := &mockStore{beads: []*model.Bead{
		makeAdvice("kd-5", "before-commit only", []string{"global"}, map[string]any{
			"hook_command":    "exit 1",
			"hook_trigger":    TriggerBeforeCommit,
			"hook_on_failure": OnFailureBlock,
		}),
	}}
	h := NewHandler(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp := h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "beads/crew/test-agent",
		Trigger: TriggerSessionEnd, // wrong trigger
	})
	if resp.Block {
		t.Error("expected no block when trigger doesn't match")
	}
}

func TestHandleSessionEvent_RigScoping(t *testing.T) {
	s := &mockStore{beads: []*model.Bead{
		makeAdvice("kd-6", "beads-rig advice", []string{"rig:beads", "global"}, map[string]any{
			"hook_command":    "echo scoped",
			"hook_trigger":    TriggerSessionEnd,
			"hook_on_failure": OnFailureBlock,
		}),
	}}
	h := NewHandler(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Agent in beads rig — should match.
	resp := h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "beads/crew/test-agent",
		Trigger: TriggerSessionEnd,
	})
	if resp.Block {
		t.Error("expected no block for matching rig (command succeeds)")
	}

	// Agent in different rig — should NOT match.
	resp = h.HandleSessionEvent(context.Background(), SessionEvent{
		AgentID: "gastown/crew/other-agent",
		Trigger: TriggerSessionEnd,
	})
	if resp.Block {
		t.Error("expected no block for non-matching rig")
	}
}
