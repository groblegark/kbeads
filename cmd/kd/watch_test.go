package main

import (
	"testing"
	"time"

	beadsv1 "github.com/groblegark/kbeads/gen/beads/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDiffBeads_InitialPoll(t *testing.T) {
	seen := make(map[string]time.Time)
	now := time.Now()
	beads := []*beadsv1.Bead{
		{Id: "a", UpdatedAt: timestamppb.New(now)},
		{Id: "b", UpdatedAt: timestamppb.New(now.Add(time.Second))},
	}

	changed := diffBeads(beads, seen)
	if len(changed) != 2 {
		t.Fatalf("got %d changed, want 2", len(changed))
	}
	if len(seen) != 2 {
		t.Fatalf("got %d seen, want 2", len(seen))
	}
}

func TestDiffBeads_NoChanges(t *testing.T) {
	now := time.Now()
	seen := map[string]time.Time{
		"a": now,
		"b": now.Add(time.Second),
	}
	beads := []*beadsv1.Bead{
		{Id: "a", UpdatedAt: timestamppb.New(now)},
		{Id: "b", UpdatedAt: timestamppb.New(now.Add(time.Second))},
	}

	changed := diffBeads(beads, seen)
	if len(changed) != 0 {
		t.Fatalf("got %d changed, want 0", len(changed))
	}
}

func TestDiffBeads_NewBead(t *testing.T) {
	now := time.Now()
	seen := map[string]time.Time{
		"a": now,
	}
	beads := []*beadsv1.Bead{
		{Id: "a", UpdatedAt: timestamppb.New(now)},
		{Id: "b", UpdatedAt: timestamppb.New(now)},
	}

	changed := diffBeads(beads, seen)
	if len(changed) != 1 {
		t.Fatalf("got %d changed, want 1", len(changed))
	}
	if changed[0].GetId() != "b" {
		t.Errorf("got changed[0].Id=%q, want %q", changed[0].GetId(), "b")
	}
}

func TestDiffBeads_UpdatedBead(t *testing.T) {
	now := time.Now()
	seen := map[string]time.Time{
		"a": now,
		"b": now,
	}
	beads := []*beadsv1.Bead{
		{Id: "a", UpdatedAt: timestamppb.New(now)},
		{Id: "b", UpdatedAt: timestamppb.New(now.Add(5 * time.Second))},
	}

	changed := diffBeads(beads, seen)
	if len(changed) != 1 {
		t.Fatalf("got %d changed, want 1", len(changed))
	}
	if changed[0].GetId() != "b" {
		t.Errorf("got changed[0].Id=%q, want %q", changed[0].GetId(), "b")
	}
	// Verify seen map was updated.
	if !seen["b"].Equal(now.Add(5 * time.Second)) {
		t.Error("seen map was not updated for bead b")
	}
}

func TestDiffBeads_NilUpdatedAt(t *testing.T) {
	seen := make(map[string]time.Time)
	beads := []*beadsv1.Bead{
		{Id: "a"}, // nil UpdatedAt
	}

	changed := diffBeads(beads, seen)
	if len(changed) != 1 {
		t.Fatalf("got %d changed, want 1", len(changed))
	}

	// Second call with same nil UpdatedAt should not diff.
	changed = diffBeads(beads, seen)
	if len(changed) != 0 {
		t.Fatalf("got %d changed on second call, want 0", len(changed))
	}
}
