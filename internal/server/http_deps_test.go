package server

import (
	"testing"

	"github.com/groblegark/kbeads/internal/model"
)

func TestAddDependency_SelfReference(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-self"] = &model.Bead{ID: "kd-self", Title: "Self", Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-self/dependencies", map[string]any{
		"depends_on_id": "kd-self", "type": "blocks",
	})
	requireStatus(t, rec, 400)
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["error"] != "cannot add dependency: bead cannot depend on itself" {
		t.Fatalf("expected self-ref error, got %q", body["error"])
	}
}

func TestAddDependency_CircularDependency(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-a"] = &model.Bead{ID: "kd-a", Title: "A", Status: model.StatusOpen}
	ms.beads["kd-b"] = &model.Bead{ID: "kd-b", Title: "B", Status: model.StatusOpen}

	// A depends on B.
	rec := doJSON(t, h, "POST", "/v1/beads/kd-a/dependencies", map[string]any{
		"depends_on_id": "kd-b", "type": "blocks",
	})
	requireStatus(t, rec, 201)

	// B depends on A — should fail (cycle).
	rec = doJSON(t, h, "POST", "/v1/beads/kd-b/dependencies", map[string]any{
		"depends_on_id": "kd-a", "type": "blocks",
	})
	requireStatus(t, rec, 400)
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["error"] != "cannot add dependency: would create a cycle" {
		t.Fatalf("expected cycle error, got %q", body["error"])
	}
}

func TestAddDependency_TransitiveCycle(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-x"] = &model.Bead{ID: "kd-x", Title: "X", Status: model.StatusOpen}
	ms.beads["kd-y"] = &model.Bead{ID: "kd-y", Title: "Y", Status: model.StatusOpen}
	ms.beads["kd-z"] = &model.Bead{ID: "kd-z", Title: "Z", Status: model.StatusOpen}

	// X → Y → Z (X depends on Y, Y depends on Z).
	rec := doJSON(t, h, "POST", "/v1/beads/kd-x/dependencies", map[string]any{
		"depends_on_id": "kd-y", "type": "blocks",
	})
	requireStatus(t, rec, 201)

	rec = doJSON(t, h, "POST", "/v1/beads/kd-y/dependencies", map[string]any{
		"depends_on_id": "kd-z", "type": "blocks",
	})
	requireStatus(t, rec, 201)

	// Z → X would create X → Y → Z → X cycle.
	rec = doJSON(t, h, "POST", "/v1/beads/kd-z/dependencies", map[string]any{
		"depends_on_id": "kd-x", "type": "blocks",
	})
	requireStatus(t, rec, 400)
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["error"] != "cannot add dependency: would create a cycle" {
		t.Fatalf("expected cycle error, got %q", body["error"])
	}
}

func TestRemoveDependency_NotFound(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-a"] = &model.Bead{ID: "kd-a", Title: "A", Status: model.StatusOpen}

	// No dependency exists between kd-a and kd-b — should return 404.
	rec := doJSON(t, h, "DELETE", "/v1/beads/kd-a/dependencies?depends_on_id=kd-b&type=blocks", nil)
	requireStatus(t, rec, 404)
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["error"] != "dependency not found" {
		t.Fatalf("expected not found error, got %q", body["error"])
	}
}

func TestAddDependency_Duplicate(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-d1"] = &model.Bead{ID: "kd-d1", Title: "A", Status: model.StatusOpen}
	ms.beads["kd-d2"] = &model.Bead{ID: "kd-d2", Title: "B", Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-d1/dependencies", map[string]any{
		"depends_on_id": "kd-d2", "type": "blocks",
	})
	requireStatus(t, rec, 201)

	// Same dependency again — should return 409.
	rec = doJSON(t, h, "POST", "/v1/beads/kd-d1/dependencies", map[string]any{
		"depends_on_id": "kd-d2", "type": "blocks",
	})
	requireStatus(t, rec, 409)
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["error"] != "dependency already exists" {
		t.Fatalf("expected duplicate error, got %q", body["error"])
	}
}
