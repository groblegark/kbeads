package server

// Tests for the decision bead type and gate system:
//
//  1. type:decision is in builtinConfigs — kd decision create no longer returns
//     "unknown bead type decision".
//  2. UpdateBead merges fields instead of replacing them — kd decision respond
//     (which only sends response_text/chosen) no longer fails "prompt: is required".
//  3. Full decision gate flow: hook emit upserts gate, responding to a decision
//     bead satisfies the gate and unblocks the Stop hook.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/groblegark/kbeads/internal/events"
	"github.com/groblegark/kbeads/internal/model"
)

// ── stateful gate mock ─────────────────────────────────────────────────────
//
// The base mockStore stubs all gate methods as no-ops. For decision tests we
// need a store that actually tracks gate state so we can verify satisfaction.

type gateKey struct{ agent, gate string }

type gatedMockStore struct {
	*mockStore
	gates map[gateKey]bool // true = satisfied
}

func newGatedStore() *gatedMockStore {
	return &gatedMockStore{
		mockStore: newMockStore(),
		gates:     make(map[gateKey]bool),
	}
}

func (g *gatedMockStore) UpsertGate(_ context.Context, agentID, gateID string) error {
	k := gateKey{agentID, gateID}
	if _, exists := g.gates[k]; !exists {
		g.gates[k] = false
	}
	return nil
}

func (g *gatedMockStore) MarkGateSatisfied(_ context.Context, agentID, gateID string) error {
	g.gates[gateKey{agentID, gateID}] = true
	return nil
}

func (g *gatedMockStore) ClearGate(_ context.Context, agentID, gateID string) error {
	delete(g.gates, gateKey{agentID, gateID})
	return nil
}

func (g *gatedMockStore) IsGateSatisfied(_ context.Context, agentID, gateID string) (bool, error) {
	return g.gates[gateKey{agentID, gateID}], nil
}

func (g *gatedMockStore) ListGates(_ context.Context, _ string) ([]model.GateRow, error) {
	return nil, nil
}

// newGatedTestServer returns a server backed by a stateful gate store.
func newGatedTestServer() (*BeadsServer, *gatedMockStore, http.Handler) {
	gs := newGatedStore()
	s := NewBeadsServer(gs, &events.NoopPublisher{})
	return s, gs, s.NewHTTPHandler("")
}

// ── type:decision in builtinConfigs ───────────────────────────────────────

// TestDecisionTypeRegistered verifies that POST /v1/beads with type=decision
// succeeds (i.e. the type config is in builtinConfigs).
func TestDecisionTypeRegistered(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "approve deploy?",
		"type":  "decision",
		"kind":  "data",
		"fields": map[string]any{
			"prompt": "Should we deploy to prod?",
		},
	})
	requireStatus(t, rec, 201)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["type"] != "decision" {
		t.Fatalf("expected type=decision, got %v", resp["type"])
	}
	if resp["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

// TestDecisionTypeConfig verifies that GET /v1/configs/type:decision returns
// the builtin config with kind=data.
func TestDecisionTypeConfig(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "GET", "/v1/configs/type:decision", nil)
	requireStatus(t, rec, 200)

	var cfg struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	decodeJSON(t, rec, &cfg)
	if cfg.Key != "type:decision" {
		t.Fatalf("expected key=type:decision, got %q", cfg.Key)
	}

	var tc map[string]any
	if err := json.Unmarshal(cfg.Value, &tc); err != nil {
		t.Fatalf("failed to decode type config value: %v", err)
	}
	if tc["kind"] != "data" {
		t.Fatalf("expected kind=data, got %v", tc["kind"])
	}
}

// ── field merge on update ──────────────────────────────────────────────────

// TestUpdateDecisionFieldsMerged verifies that PATCH /v1/beads/{id} merges
// the incoming fields into existing ones rather than replacing them.
// This is the fix for kd decision respond failing "prompt: is required".
func TestUpdateDecisionFieldsMerged(t *testing.T) {
	_, ms, h := newTestServer()

	// Create a decision bead with prompt and options already set.
	createRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "deploy?",
		"type":  "decision",
		"kind":  "data",
		"fields": map[string]any{
			"prompt":  "Deploy to production?",
			"options": []map[string]any{{"id": "y", "label": "Yes"}, {"id": "n", "label": "No"}},
		},
	})
	requireStatus(t, createRec, 201)

	var created map[string]any
	decodeJSON(t, createRec, &created)
	id := created["id"].(string)

	// Now update with only the response fields (simulating kd decision respond).
	updateRec := doJSON(t, h, "PATCH", "/v1/beads/"+id, map[string]any{
		"fields": map[string]any{
			"chosen":       "y",
			"responded_by": "alice",
		},
	})
	requireStatus(t, updateRec, 200)

	// Verify the updated bead still has the original prompt AND the new response.
	bead := ms.beads[id]
	if bead == nil {
		t.Fatalf("bead %s not found in store", id)
	}
	var fields map[string]any
	if err := json.Unmarshal(bead.Fields, &fields); err != nil {
		t.Fatalf("failed to decode bead fields: %v", err)
	}
	if fields["prompt"] != "Deploy to production?" {
		t.Errorf("prompt field overwritten; got %v", fields["prompt"])
	}
	if fields["chosen"] != "y" {
		t.Errorf("chosen field not set; got %v", fields["chosen"])
	}
	if fields["responded_by"] != "alice" {
		t.Errorf("responded_by field not set; got %v", fields["responded_by"])
	}
}

// ── decision gate: full flow ───────────────────────────────────────────────

// TestDecisionGateFlow exercises the complete decision gate lifecycle:
//  1. POST /v1/hooks/emit with hook_type=Stop → gate is pending → blocked
//  2. POST /v1/beads (decision) sets requesting_agent_bead_id
//  3. POST /v1/decisions/{id}/resolve → gate is satisfied
//  4. POST /v1/hooks/emit with hook_type=Stop → now unblocked
func TestDecisionGateFlow(t *testing.T) {
	_, gs, h := newGatedTestServer()

	const agentID = "kd-agent-test"

	// Step 1: Emit Stop hook → gate is pending → response should be blocked.
	stopRec := doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id":     agentID,
		"hook_type":         "Stop",
		"claude_session_id": "sess-abc",
		"cwd":               "/workspace",
		"actor":             "test-agent",
	})
	requireStatus(t, stopRec, 200)
	var stopResp1 map[string]any
	decodeJSON(t, stopRec, &stopResp1)
	if stopResp1["block"] != true {
		t.Fatalf("expected block=true on first Stop, got %v", stopResp1)
	}

	// Step 2: Create a decision bead referencing the agent.
	decisionRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "approve shutdown?",
		"type":  "decision",
		"kind":  "data",
		"fields": map[string]any{
			"prompt":                   "Can the agent shut down?",
			"requesting_agent_bead_id": agentID,
		},
	})
	requireStatus(t, decisionRec, 201)
	var decisionBead map[string]any
	decodeJSON(t, decisionRec, &decisionBead)
	decisionID := decisionBead["id"].(string)

	// Gate should still be pending (decision not yet resolved).
	if gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should not be satisfied yet")
	}

	// Step 3: Resolve the decision → gate should be satisfied.
	resolveRec := doJSON(t, h, "POST", "/v1/decisions/"+decisionID+"/resolve", map[string]any{
		"selected_option": "y",
		"responded_by":    "test-agent",
	})
	requireStatus(t, resolveRec, 200)

	if !gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should be satisfied after decision resolved")
	}

	// Step 4: Emit Stop hook again → gate is now satisfied → unblocked.
	stopRec2 := doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id":     agentID,
		"hook_type":         "Stop",
		"claude_session_id": "sess-abc",
		"cwd":               "/workspace",
		"actor":             "test-agent",
	})
	requireStatus(t, stopRec2, 200)
	var stopResp2 map[string]any
	decodeJSON(t, stopRec2, &stopResp2)
	if stopResp2["block"] == true {
		t.Fatalf("expected unblocked on second Stop after decision resolved, got %v", stopResp2)
	}
}

// TestDecisionGateSatisfiedByClose verifies that closing a decision bead
// (rather than using the /resolve endpoint) also satisfies the gate.
func TestDecisionGateSatisfiedByClose(t *testing.T) {
	_, gs, h := newGatedTestServer()

	const agentID = "kd-agent-close-test"

	// Emit Stop to register the gate.
	stopRec := doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id": agentID,
		"hook_type":     "Stop",
		"actor":         "test-agent",
	})
	requireStatus(t, stopRec, 200)
	var stopResp map[string]any
	decodeJSON(t, stopRec, &stopResp)
	if stopResp["block"] != true {
		t.Fatalf("expected block=true, got %v", stopResp)
	}

	// Create and close a decision bead for this agent.
	decisionRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "ok to close?",
		"type":  "decision",
		"kind":  "data",
		"fields": map[string]any{
			"prompt":                   "Proceed?",
			"requesting_agent_bead_id": agentID,
		},
	})
	requireStatus(t, decisionRec, 201)
	var decisionBead map[string]any
	decodeJSON(t, decisionRec, &decisionBead)
	decisionID := decisionBead["id"].(string)

	// Close via POST /v1/beads/{id}/close.
	closeRec := doJSON(t, h, "POST", "/v1/beads/"+decisionID+"/close", map[string]any{
		"closed_by": "test-agent",
	})
	requireStatus(t, closeRec, 200)

	if !gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should be satisfied after closing decision bead")
	}
}

// ── report-gated decision flow ─────────────────────────────────────────

// TestDecisionReportGatedFlow exercises the complete report-gated lifecycle:
//  1. Create decision with "report:summary" label
//  2. Resolve the decision → gate stays pending (report required)
//  3. Create and close a report bead → gate is satisfied
func TestDecisionReportGatedFlow(t *testing.T) {
	_, gs, h := newGatedTestServer()

	const agentID = "kd-agent-report-test"

	// Step 1: Emit Stop hook → gate pending → blocked.
	stopRec := doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id": agentID,
		"hook_type":     "Stop",
		"actor":         "test-agent",
	})
	requireStatus(t, stopRec, 200)
	var stopResp map[string]any
	decodeJSON(t, stopRec, &stopResp)
	if stopResp["block"] != true {
		t.Fatalf("expected block=true on Stop, got %v", stopResp)
	}

	// Step 2: Create decision with report:summary label.
	decisionRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title":  "deploy approval?",
		"type":   "decision",
		"kind":   "data",
		"labels": []string{"report:summary"},
		"fields": map[string]any{
			"prompt":                   "Approve deploy to prod?",
			"requesting_agent_bead_id": agentID,
		},
	})
	requireStatus(t, decisionRec, 201)
	var decisionBead map[string]any
	decodeJSON(t, decisionRec, &decisionBead)
	decisionID := decisionBead["id"].(string)

	// Step 3: Resolve the decision.
	resolveRec := doJSON(t, h, "POST", "/v1/decisions/"+decisionID+"/resolve", map[string]any{
		"selected_option": "approved",
		"responded_by":    "human",
	})
	requireStatus(t, resolveRec, 200)

	// Verify resolve response includes report_required.
	var resolveResp map[string]any
	decodeJSON(t, resolveRec, &resolveResp)
	if resolveResp["report_required"] != true {
		t.Fatalf("expected report_required=true in resolve response, got %v", resolveResp["report_required"])
	}
	if resolveResp["report_type"] != "summary" {
		t.Fatalf("expected report_type=summary, got %v", resolveResp["report_type"])
	}

	// Gate should still be pending — report not yet submitted.
	if gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should NOT be satisfied after resolve (report required)")
	}

	// Step 4: Create report bead linked to the decision.
	reportRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "Deploy summary report",
		"type":  "report",
		"kind":  "data",
		"fields": map[string]any{
			"decision_id": decisionID,
			"report_type": "summary",
			"content":     "All systems verified. Deploy successful.",
		},
	})
	requireStatus(t, reportRec, 201)
	var reportBead map[string]any
	decodeJSON(t, reportRec, &reportBead)
	reportID := reportBead["id"].(string)

	// Gate still pending — report not yet closed.
	if gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should NOT be satisfied before report is closed")
	}

	// Step 5: Close the report bead → gate should be satisfied.
	closeRec := doJSON(t, h, "POST", "/v1/beads/"+reportID+"/close", map[string]any{
		"closed_by": "agent",
	})
	requireStatus(t, closeRec, 200)

	if !gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should be satisfied after report bead closed")
	}

	// Step 6: Stop hook should now be unblocked.
	stopRec2 := doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id": agentID,
		"hook_type":     "Stop",
		"actor":         "test-agent",
	})
	requireStatus(t, stopRec2, 200)
	var stopResp2 map[string]any
	decodeJSON(t, stopRec2, &stopResp2)
	if stopResp2["block"] == true {
		t.Fatalf("expected unblocked after report submitted, got %v", stopResp2)
	}
}

// TestDecisionWithoutReportLabel verifies decisions without report: labels
// still satisfy the gate immediately on resolve (no regression).
func TestDecisionWithoutReportLabel(t *testing.T) {
	_, gs, h := newGatedTestServer()

	const agentID = "kd-agent-no-report"

	// Register gate.
	doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id": agentID,
		"hook_type":     "Stop",
		"actor":         "test-agent",
	})

	// Create decision WITHOUT report label.
	decisionRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "simple decision",
		"type":  "decision",
		"kind":  "data",
		"fields": map[string]any{
			"prompt":                   "Continue?",
			"requesting_agent_bead_id": agentID,
		},
	})
	requireStatus(t, decisionRec, 201)
	var bead map[string]any
	decodeJSON(t, decisionRec, &bead)
	decisionID := bead["id"].(string)

	// Resolve — gate should be satisfied immediately.
	resolveRec := doJSON(t, h, "POST", "/v1/decisions/"+decisionID+"/resolve", map[string]any{
		"selected_option": "yes",
		"responded_by":    "human",
	})
	requireStatus(t, resolveRec, 200)

	// Verify resolve response does NOT include report_required.
	var resolveResp map[string]any
	decodeJSON(t, resolveRec, &resolveResp)
	if resolveResp["report_required"] != nil {
		t.Fatalf("expected no report_required for decision without label, got %v", resolveResp["report_required"])
	}

	if !gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should be satisfied immediately after resolve (no report label)")
	}
}

// TestReportGateSatisfiedByGenericClose verifies that closing a decision bead
// with a report: label via POST /v1/beads/{id}/close also skips gate satisfaction.
func TestReportGateSatisfiedByGenericClose(t *testing.T) {
	_, gs, h := newGatedTestServer()

	const agentID = "kd-agent-generic-close"

	// Register gate.
	doJSON(t, h, "POST", "/v1/hooks/emit", map[string]any{
		"agent_bead_id": agentID,
		"hook_type":     "Stop",
		"actor":         "test-agent",
	})

	// Create decision with report label.
	decisionRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title":  "report-gated close test",
		"type":   "decision",
		"kind":   "data",
		"labels": []string{"report:deploy-checklist"},
		"fields": map[string]any{
			"prompt":                   "Ready?",
			"requesting_agent_bead_id": agentID,
		},
	})
	requireStatus(t, decisionRec, 201)
	var bead map[string]any
	decodeJSON(t, decisionRec, &bead)
	decisionID := bead["id"].(string)

	// Close via generic close endpoint.
	closeRec := doJSON(t, h, "POST", "/v1/beads/"+decisionID+"/close", map[string]any{
		"closed_by": "human",
		"chosen":    "approved",
	})
	requireStatus(t, closeRec, 200)

	// Gate should NOT be satisfied — report required.
	if gs.gates[gateKey{agentID, "decision"}] {
		t.Fatal("gate should NOT be satisfied after closing report-gated decision")
	}
}

// TestReportTypeRegistered verifies POST /v1/beads with type=report succeeds.
func TestReportTypeRegistered(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "test report",
		"type":  "report",
		"kind":  "data",
		"fields": map[string]any{
			"decision_id": "kd-decision-123",
			"report_type": "summary",
			"content":     "Report content here.",
		},
	})
	requireStatus(t, rec, 201)

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["type"] != "report" {
		t.Fatalf("expected type=report, got %v", resp["type"])
	}
}

// TestDecisionExtractFieldsReportRequired verifies that extractDecisionFields
// includes report_required and report_type when a report: label is present.
func TestDecisionExtractFieldsReportRequired(t *testing.T) {
	_, _, h := newTestServer()

	// Create decision with report label.
	decisionRec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title":  "decision with report",
		"type":   "decision",
		"kind":   "data",
		"labels": []string{"report:code-review"},
		"fields": map[string]any{
			"prompt": "Review needed?",
		},
	})
	requireStatus(t, decisionRec, 201)
	var bead map[string]any
	decodeJSON(t, decisionRec, &bead)
	decisionID := bead["id"].(string)

	// GET /v1/decisions/{id} should include report fields.
	getRec := doJSON(t, h, "GET", "/v1/decisions/"+decisionID, nil)
	requireStatus(t, getRec, 200)

	var resp struct {
		Decision map[string]any `json:"decision"`
	}
	decodeJSON(t, getRec, &resp)

	if resp.Decision["report_required"] != true {
		t.Fatalf("expected report_required=true, got %v", resp.Decision["report_required"])
	}
	if resp.Decision["report_type"] != "code-review" {
		t.Fatalf("expected report_type=code-review, got %v", resp.Decision["report_type"])
	}
}

// TestDecisionCreateUnknownTypeGone verifies the old "unknown bead type" error
// no longer occurs — regression test for the original bug.
func TestDecisionCreateUnknownTypeGone(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title":  "should this work?",
		"type":   "decision",
		"kind":   "data",
		"fields": map[string]any{"prompt": "yes?"},
	})
	// Must NOT be 400 "unknown bead type decision".
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("got 400 (unknown bead type); regression: %s", rec.Body.String())
	}
	requireStatus(t, rec, 201)
}
