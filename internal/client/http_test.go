package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/groblegark/kbeads/internal/model"
)

// testHandler captures the incoming request details and returns a canned response.
type testHandler struct {
	// captured from the request
	method      string
	path        string
	rawPath     string // URL-encoded path (for testing PathEscape)
	requestURI  string
	query       string
	body        string
	contentType string

	// canned response
	statusCode   int
	responseBody string
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.method = r.Method
	h.path = r.URL.Path
	h.rawPath = r.URL.RawPath
	h.requestURI = r.RequestURI
	h.query = r.URL.RawQuery
	h.contentType = r.Header.Get("Content-Type")
	if r.Body != nil {
		data, _ := io.ReadAll(r.Body)
		h.body = string(data)
	}

	w.Header().Set("Content-Type", "application/json")
	if h.statusCode != 0 {
		w.WriteHeader(h.statusCode)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if h.responseBody != "" {
		_, _ = w.Write([]byte(h.responseBody))
	}
}

// newTestClient creates an HTTPClient pointed at a test server with the given handler.
func newTestClient(h http.Handler) (*HTTPClient, *httptest.Server) {
	srv := httptest.NewServer(h)
	c := NewHTTPClient(srv.URL)
	return c, srv
}

// --- CreateBead ---

func TestHTTPClient_CreateBead(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"id": "bead-abc",
			"slug": "bd-abc",
			"kind": "issue",
			"type": "task",
			"title": "Fix the widget",
			"description": "It is broken",
			"status": "open",
			"priority": 2,
			"assignee": "alice",
			"owner": "bob",
			"created_by": "alice",
			"created_at": "2026-01-15T10:00:00Z",
			"updated_at": "2026-01-15T10:00:00Z",
			"labels": ["urgent", "backend"]
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	due := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	req := &CreateBeadRequest{
		Title:       "Fix the widget",
		Kind:        "issue",
		Type:        "task",
		Description: "It is broken",
		Priority:    2,
		Assignee:    "alice",
		Owner:       "bob",
		Labels:      []string{"urgent", "backend"},
		CreatedBy:   "alice",
		DueAt:       &due,
	}

	bead, err := c.CreateBead(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateBead() error = %v", err)
	}

	// Verify request
	if h.method != http.MethodPost {
		t.Errorf("method = %q, want POST", h.method)
	}
	if h.path != "/v1/beads" {
		t.Errorf("path = %q, want /v1/beads", h.path)
	}
	if h.contentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", h.contentType)
	}

	// Verify request body contains expected fields
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["title"] != "Fix the widget" {
		t.Errorf("request body title = %v, want 'Fix the widget'", reqBody["title"])
	}
	if reqBody["type"] != "task" {
		t.Errorf("request body type = %v, want 'task'", reqBody["type"])
	}
	if reqBody["assignee"] != "alice" {
		t.Errorf("request body assignee = %v, want 'alice'", reqBody["assignee"])
	}
	if reqBody["due_at"] == nil {
		t.Error("request body due_at is nil, want non-nil")
	}

	// Verify response parsing
	if bead.ID != "bead-abc" {
		t.Errorf("bead.ID = %q, want 'bead-abc'", bead.ID)
	}
	if bead.Slug != "bd-abc" {
		t.Errorf("bead.Slug = %q, want 'bd-abc'", bead.Slug)
	}
	if bead.Kind != model.KindIssue {
		t.Errorf("bead.Kind = %q, want 'issue'", bead.Kind)
	}
	if bead.Type != model.TypeTask {
		t.Errorf("bead.Type = %q, want 'task'", bead.Type)
	}
	if bead.Title != "Fix the widget" {
		t.Errorf("bead.Title = %q, want 'Fix the widget'", bead.Title)
	}
	if bead.Priority != 2 {
		t.Errorf("bead.Priority = %d, want 2", bead.Priority)
	}
	if bead.Assignee != "alice" {
		t.Errorf("bead.Assignee = %q, want 'alice'", bead.Assignee)
	}
	if len(bead.Labels) != 2 {
		t.Errorf("len(bead.Labels) = %d, want 2", len(bead.Labels))
	}
}

func TestHTTPClient_CreateBead_MinimalFields(t *testing.T) {
	h := &testHandler{
		responseBody: `{"id": "bead-min", "title": "Minimal", "type": "task", "kind": "issue", "status": "open", "priority": 0, "created_at": "2026-01-15T10:00:00Z", "updated_at": "2026-01-15T10:00:00Z"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	req := &CreateBeadRequest{
		Title: "Minimal",
		Type:  "task",
	}

	bead, err := c.CreateBead(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateBead() error = %v", err)
	}
	if bead.ID != "bead-min" {
		t.Errorf("bead.ID = %q, want 'bead-min'", bead.ID)
	}

	// Verify omitempty fields are absent from request body
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if _, ok := reqBody["assignee"]; ok {
		t.Error("request body should not contain 'assignee' when empty")
	}
	if _, ok := reqBody["due_at"]; ok {
		t.Error("request body should not contain 'due_at' when nil")
	}
	if _, ok := reqBody["labels"]; ok {
		t.Error("request body should not contain 'labels' when nil")
	}
}

// --- GetBead ---

func TestHTTPClient_GetBead(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"id": "bead-123",
			"slug": "bd-123",
			"kind": "issue",
			"type": "bug",
			"title": "Something broke",
			"status": "open",
			"priority": 1,
			"created_at": "2026-01-10T08:00:00Z",
			"updated_at": "2026-01-11T09:30:00Z",
			"closed_at": "2026-01-12T12:00:00Z",
			"closed_by": "bob",
			"fields": {"severity": "high"}
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	bead, err := c.GetBead(context.Background(), "bead-123")
	if err != nil {
		t.Fatalf("GetBead() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/beads/bead-123" {
		t.Errorf("path = %q, want /v1/beads/bead-123", h.path)
	}
	if h.contentType != "" {
		t.Errorf("GET should not have Content-Type, got %q", h.contentType)
	}

	if bead.ID != "bead-123" {
		t.Errorf("bead.ID = %q, want 'bead-123'", bead.ID)
	}
	if bead.Type != model.TypeBug {
		t.Errorf("bead.Type = %q, want 'bug'", bead.Type)
	}
	if bead.ClosedAt == nil {
		t.Error("bead.ClosedAt is nil, want non-nil")
	}
	if bead.ClosedBy != "bob" {
		t.Errorf("bead.ClosedBy = %q, want 'bob'", bead.ClosedBy)
	}
	if bead.Fields == nil {
		t.Error("bead.Fields is nil, want non-nil")
	}
}

func TestHTTPClient_GetBead_URLEscaping(t *testing.T) {
	h := &testHandler{
		responseBody: `{"id": "bead/special", "title": "Test", "type": "task", "kind": "issue", "status": "open", "priority": 0, "created_at": "2026-01-15T10:00:00Z", "updated_at": "2026-01-15T10:00:00Z"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	_, err := c.GetBead(context.Background(), "bead/special")
	if err != nil {
		t.Fatalf("GetBead() error = %v", err)
	}

	// The slash in the ID should be URL-escaped on the wire.
	// r.URL.Path is decoded by the Go HTTP server, so we check requestURI.
	wantURI := "/v1/beads/bead%2Fspecial"
	if h.requestURI != wantURI {
		t.Errorf("requestURI = %q, want %q", h.requestURI, wantURI)
	}
}

// --- ListBeads ---

func TestHTTPClient_ListBeads(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"beads": [
				{"id": "b1", "title": "First", "type": "task", "kind": "issue", "status": "open", "priority": 1, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
				{"id": "b2", "title": "Second", "type": "bug", "kind": "issue", "status": "closed", "priority": 2, "created_at": "2026-01-02T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z"}
			],
			"total": 42
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	prio := 1
	resp, err := c.ListBeads(context.Background(), &ListBeadsRequest{
		Status:   []string{"open", "in_progress"},
		Type:     []string{"task", "bug"},
		Kind:     []string{"issue"},
		Labels:   []string{"urgent", "backend"},
		Assignee: "alice",
		Search:   "widget",
		Sort:     "-priority",
		Priority: &prio,
		Limit:    10,
		Offset:   20,
	})
	if err != nil {
		t.Fatalf("ListBeads() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/beads" {
		t.Errorf("path = %q, want /v1/beads", h.path)
	}

	// Verify query params are present
	q := h.query
	for _, want := range []string{
		"status=open%2Cin_progress",
		"type=task%2Cbug",
		"kind=issue",
		"labels=urgent%2Cbackend",
		"assignee=alice",
		"search=widget",
		"sort=-priority",
		"priority=1",
		"limit=10",
		"offset=20",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q does not contain %q", q, want)
		}
	}

	// Verify response parsing
	if len(resp.Beads) != 2 {
		t.Fatalf("len(beads) = %d, want 2", len(resp.Beads))
	}
	if resp.Total != 42 {
		t.Errorf("total = %d, want 42", resp.Total)
	}
	if resp.Beads[0].ID != "b1" {
		t.Errorf("beads[0].ID = %q, want 'b1'", resp.Beads[0].ID)
	}
	if resp.Beads[1].Status != model.StatusClosed {
		t.Errorf("beads[1].Status = %q, want 'closed'", resp.Beads[1].Status)
	}
}

func TestHTTPClient_ListBeads_NoFilters(t *testing.T) {
	h := &testHandler{
		responseBody: `{"beads": [], "total": 0}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	resp, err := c.ListBeads(context.Background(), &ListBeadsRequest{})
	if err != nil {
		t.Fatalf("ListBeads() error = %v", err)
	}

	// No query params should be set
	if h.query != "" {
		t.Errorf("query = %q, want empty", h.query)
	}
	if len(resp.Beads) != 0 {
		t.Errorf("len(beads) = %d, want 0", len(resp.Beads))
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
}

func TestHTTPClient_ListBeads_EmptyResponse(t *testing.T) {
	h := &testHandler{
		responseBody: `{"beads": null, "total": 0}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	resp, err := c.ListBeads(context.Background(), &ListBeadsRequest{})
	if err != nil {
		t.Fatalf("ListBeads() error = %v", err)
	}

	if resp.Beads != nil {
		t.Errorf("beads = %v, want nil", resp.Beads)
	}
}

// --- UpdateBead ---

func TestHTTPClient_UpdateBead(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"id": "bead-upd",
			"slug": "bd-upd",
			"kind": "issue",
			"type": "task",
			"title": "Updated title",
			"description": "Updated desc",
			"status": "in_progress",
			"priority": 3,
			"assignee": "carol",
			"created_at": "2026-01-15T10:00:00Z",
			"updated_at": "2026-01-16T14:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	title := "Updated title"
	desc := "Updated desc"
	status := "in_progress"
	prio := 3
	assignee := "carol"

	bead, err := c.UpdateBead(context.Background(), "bead-upd", &UpdateBeadRequest{
		Title:       &title,
		Description: &desc,
		Status:      &status,
		Priority:    &prio,
		Assignee:    &assignee,
	})
	if err != nil {
		t.Fatalf("UpdateBead() error = %v", err)
	}

	if h.method != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", h.method)
	}
	if h.path != "/v1/beads/bead-upd" {
		t.Errorf("path = %q, want /v1/beads/bead-upd", h.path)
	}

	// Verify request body has only the set fields
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["title"] != "Updated title" {
		t.Errorf("request body title = %v, want 'Updated title'", reqBody["title"])
	}
	if _, ok := reqBody["notes"]; ok {
		t.Error("request body should not contain 'notes' when nil")
	}
	if _, ok := reqBody["owner"]; ok {
		t.Error("request body should not contain 'owner' when nil")
	}

	if bead.Title != "Updated title" {
		t.Errorf("bead.Title = %q, want 'Updated title'", bead.Title)
	}
	if bead.Status != model.StatusInProgress {
		t.Errorf("bead.Status = %q, want 'in_progress'", bead.Status)
	}
}

func TestHTTPClient_UpdateBead_OnlyNotes(t *testing.T) {
	h := &testHandler{
		responseBody: `{"id": "bead-n", "title": "X", "type": "task", "kind": "issue", "status": "open", "notes": "new notes", "priority": 0, "created_at": "2026-01-15T10:00:00Z", "updated_at": "2026-01-15T10:00:00Z"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	notes := "new notes"
	bead, err := c.UpdateBead(context.Background(), "bead-n", &UpdateBeadRequest{
		Notes: &notes,
	})
	if err != nil {
		t.Fatalf("UpdateBead() error = %v", err)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["notes"] != "new notes" {
		t.Errorf("request body notes = %v, want 'new notes'", reqBody["notes"])
	}
	// title, description, status, etc. should not be in the body
	if _, ok := reqBody["title"]; ok {
		t.Error("request body should not contain 'title' when nil")
	}
	if bead.Notes != "new notes" {
		t.Errorf("bead.Notes = %q, want 'new notes'", bead.Notes)
	}
}

// --- CloseBead ---

func TestHTTPClient_CloseBead(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"id": "bead-cls",
			"title": "Closed bead",
			"type": "task",
			"kind": "issue",
			"status": "closed",
			"priority": 1,
			"closed_at": "2026-01-20T15:00:00Z",
			"closed_by": "alice",
			"created_at": "2026-01-15T10:00:00Z",
			"updated_at": "2026-01-20T15:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	bead, err := c.CloseBead(context.Background(), "bead-cls", "alice")
	if err != nil {
		t.Fatalf("CloseBead() error = %v", err)
	}

	if h.method != http.MethodPost {
		t.Errorf("method = %q, want POST", h.method)
	}
	if h.path != "/v1/beads/bead-cls/close" {
		t.Errorf("path = %q, want /v1/beads/bead-cls/close", h.path)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["closed_by"] != "alice" {
		t.Errorf("request body closed_by = %v, want 'alice'", reqBody["closed_by"])
	}

	if bead.Status != model.StatusClosed {
		t.Errorf("bead.Status = %q, want 'closed'", bead.Status)
	}
	if bead.ClosedAt == nil {
		t.Error("bead.ClosedAt is nil, want non-nil")
	}
}

func TestHTTPClient_CloseBead_EmptyClosedBy(t *testing.T) {
	h := &testHandler{
		responseBody: `{"id": "bead-cls2", "title": "X", "type": "task", "kind": "issue", "status": "closed", "priority": 0, "created_at": "2026-01-15T10:00:00Z", "updated_at": "2026-01-15T10:00:00Z"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	_, err := c.CloseBead(context.Background(), "bead-cls2", "")
	if err != nil {
		t.Fatalf("CloseBead() error = %v", err)
	}

	// When closedBy is empty, body should be an empty map (no closed_by key)
	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if _, ok := reqBody["closed_by"]; ok {
		t.Error("request body should not contain 'closed_by' when empty")
	}
}

// --- DeleteBead ---

func TestHTTPClient_DeleteBead(t *testing.T) {
	h := &testHandler{
		statusCode: http.StatusNoContent,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.DeleteBead(context.Background(), "bead-del")
	if err != nil {
		t.Fatalf("DeleteBead() error = %v", err)
	}

	if h.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", h.method)
	}
	if h.path != "/v1/beads/bead-del" {
		t.Errorf("path = %q, want /v1/beads/bead-del", h.path)
	}
}

// --- AddDependency ---

func TestHTTPClient_AddDependency(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"bead_id": "bead-a",
			"depends_on_id": "bead-b",
			"type": "blocks",
			"created_by": "alice",
			"created_at": "2026-01-15T10:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	dep, err := c.AddDependency(context.Background(), &AddDependencyRequest{
		BeadID:      "bead-a",
		DependsOnID: "bead-b",
		Type:        "blocks",
		CreatedBy:   "alice",
	})
	if err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	if h.method != http.MethodPost {
		t.Errorf("method = %q, want POST", h.method)
	}
	if h.path != "/v1/beads/bead-a/dependencies" {
		t.Errorf("path = %q, want /v1/beads/bead-a/dependencies", h.path)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["depends_on_id"] != "bead-b" {
		t.Errorf("request body depends_on_id = %v, want 'bead-b'", reqBody["depends_on_id"])
	}
	if reqBody["type"] != "blocks" {
		t.Errorf("request body type = %v, want 'blocks'", reqBody["type"])
	}

	if dep.BeadID != "bead-a" {
		t.Errorf("dep.BeadID = %q, want 'bead-a'", dep.BeadID)
	}
	if dep.DependsOnID != "bead-b" {
		t.Errorf("dep.DependsOnID = %q, want 'bead-b'", dep.DependsOnID)
	}
	if dep.Type != model.DepBlocks {
		t.Errorf("dep.Type = %q, want 'blocks'", dep.Type)
	}
}

// --- RemoveDependency ---

func TestHTTPClient_RemoveDependency(t *testing.T) {
	h := &testHandler{
		statusCode: http.StatusNoContent,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.RemoveDependency(context.Background(), "bead-a", "bead-b", "blocks")
	if err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}

	if h.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", h.method)
	}
	if h.path != "/v1/beads/bead-a/dependencies" {
		t.Errorf("path = %q, want /v1/beads/bead-a/dependencies", h.path)
	}

	// Verify query params
	q := h.query
	if !strings.Contains(q, "depends_on_id=bead-b") {
		t.Errorf("query %q missing depends_on_id=bead-b", q)
	}
	if !strings.Contains(q, "type=blocks") {
		t.Errorf("query %q missing type=blocks", q)
	}
}

func TestHTTPClient_RemoveDependency_NoType(t *testing.T) {
	h := &testHandler{
		statusCode: http.StatusNoContent,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.RemoveDependency(context.Background(), "bead-a", "bead-b", "")
	if err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}

	q := h.query
	if !strings.Contains(q, "depends_on_id=bead-b") {
		t.Errorf("query %q missing depends_on_id=bead-b", q)
	}
	if strings.Contains(q, "type=") {
		t.Errorf("query %q should not contain type= when depType is empty", q)
	}
}

// --- GetDependencies ---

func TestHTTPClient_GetDependencies(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"dependencies": [
				{"bead_id": "bead-a", "depends_on_id": "bead-b", "type": "blocks", "created_at": "2026-01-15T10:00:00Z"},
				{"bead_id": "bead-a", "depends_on_id": "bead-c", "type": "related", "created_at": "2026-01-16T10:00:00Z"}
			]
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	deps, err := c.GetDependencies(context.Background(), "bead-a")
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/beads/bead-a/dependencies" {
		t.Errorf("path = %q, want /v1/beads/bead-a/dependencies", h.path)
	}

	if len(deps) != 2 {
		t.Fatalf("len(deps) = %d, want 2", len(deps))
	}
	if deps[0].DependsOnID != "bead-b" {
		t.Errorf("deps[0].DependsOnID = %q, want 'bead-b'", deps[0].DependsOnID)
	}
	if deps[1].Type != model.DepRelated {
		t.Errorf("deps[1].Type = %q, want 'related'", deps[1].Type)
	}
}

func TestHTTPClient_GetDependencies_Empty(t *testing.T) {
	h := &testHandler{
		responseBody: `{"dependencies": []}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	deps, err := c.GetDependencies(context.Background(), "bead-lonely")
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}

	if len(deps) != 0 {
		t.Errorf("len(deps) = %d, want 0", len(deps))
	}
}

// --- AddLabel ---

func TestHTTPClient_AddLabel(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"id": "bead-lbl",
			"title": "Labeled",
			"type": "task",
			"kind": "issue",
			"status": "open",
			"priority": 1,
			"labels": ["urgent", "backend"],
			"created_at": "2026-01-15T10:00:00Z",
			"updated_at": "2026-01-15T10:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	bead, err := c.AddLabel(context.Background(), "bead-lbl", "backend")
	if err != nil {
		t.Fatalf("AddLabel() error = %v", err)
	}

	if h.method != http.MethodPost {
		t.Errorf("method = %q, want POST", h.method)
	}
	if h.path != "/v1/beads/bead-lbl/labels" {
		t.Errorf("path = %q, want /v1/beads/bead-lbl/labels", h.path)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["label"] != "backend" {
		t.Errorf("request body label = %v, want 'backend'", reqBody["label"])
	}

	if len(bead.Labels) != 2 {
		t.Errorf("len(bead.Labels) = %d, want 2", len(bead.Labels))
	}
}

// --- RemoveLabel ---

func TestHTTPClient_RemoveLabel(t *testing.T) {
	h := &testHandler{
		statusCode: http.StatusNoContent,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.RemoveLabel(context.Background(), "bead-lbl", "backend")
	if err != nil {
		t.Fatalf("RemoveLabel() error = %v", err)
	}

	if h.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", h.method)
	}
	if h.path != "/v1/beads/bead-lbl/labels/backend" {
		t.Errorf("path = %q, want /v1/beads/bead-lbl/labels/backend", h.path)
	}
}

func TestHTTPClient_RemoveLabel_SpecialChars(t *testing.T) {
	h := &testHandler{
		statusCode: http.StatusNoContent,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.RemoveLabel(context.Background(), "bead-lbl", "team/backend")
	if err != nil {
		t.Fatalf("RemoveLabel() error = %v", err)
	}

	// Slash in label should be URL-escaped on the wire.
	// r.URL.Path is decoded by the Go HTTP server, so we check requestURI.
	wantURI := "/v1/beads/bead-lbl/labels/team%2Fbackend"
	if h.requestURI != wantURI {
		t.Errorf("requestURI = %q, want %q", h.requestURI, wantURI)
	}
}

// --- GetLabels ---

func TestHTTPClient_GetLabels(t *testing.T) {
	h := &testHandler{
		responseBody: `{"labels": ["urgent", "backend", "P0"]}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	labels, err := c.GetLabels(context.Background(), "bead-lbl")
	if err != nil {
		t.Fatalf("GetLabels() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/beads/bead-lbl/labels" {
		t.Errorf("path = %q, want /v1/beads/bead-lbl/labels", h.path)
	}

	if len(labels) != 3 {
		t.Fatalf("len(labels) = %d, want 3", len(labels))
	}
	if labels[0] != "urgent" {
		t.Errorf("labels[0] = %q, want 'urgent'", labels[0])
	}
	if labels[2] != "P0" {
		t.Errorf("labels[2] = %q, want 'P0'", labels[2])
	}
}

func TestHTTPClient_GetLabels_Empty(t *testing.T) {
	h := &testHandler{
		responseBody: `{"labels": []}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	labels, err := c.GetLabels(context.Background(), "bead-no-labels")
	if err != nil {
		t.Fatalf("GetLabels() error = %v", err)
	}
	if len(labels) != 0 {
		t.Errorf("len(labels) = %d, want 0", len(labels))
	}
}

// --- AddComment ---

func TestHTTPClient_AddComment(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"id": 42,
			"bead_id": "bead-cmt",
			"author": "alice",
			"text": "This looks good!",
			"created_at": "2026-01-15T10:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	comment, err := c.AddComment(context.Background(), "bead-cmt", "alice", "This looks good!")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	if h.method != http.MethodPost {
		t.Errorf("method = %q, want POST", h.method)
	}
	if h.path != "/v1/beads/bead-cmt/comments" {
		t.Errorf("path = %q, want /v1/beads/bead-cmt/comments", h.path)
	}

	var reqBody map[string]interface{}
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	if reqBody["author"] != "alice" {
		t.Errorf("request body author = %v, want 'alice'", reqBody["author"])
	}
	if reqBody["text"] != "This looks good!" {
		t.Errorf("request body text = %v, want 'This looks good!'", reqBody["text"])
	}

	if comment.ID != 42 {
		t.Errorf("comment.ID = %d, want 42", comment.ID)
	}
	if comment.BeadID != "bead-cmt" {
		t.Errorf("comment.BeadID = %q, want 'bead-cmt'", comment.BeadID)
	}
	if comment.Author != "alice" {
		t.Errorf("comment.Author = %q, want 'alice'", comment.Author)
	}
	if comment.Text != "This looks good!" {
		t.Errorf("comment.Text = %q, want 'This looks good!'", comment.Text)
	}
}

// --- GetComments ---

func TestHTTPClient_GetComments(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"comments": [
				{"id": 1, "bead_id": "bead-cmt", "author": "alice", "text": "First", "created_at": "2026-01-15T10:00:00Z"},
				{"id": 2, "bead_id": "bead-cmt", "author": "bob", "text": "Second", "created_at": "2026-01-15T11:00:00Z"}
			]
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	comments, err := c.GetComments(context.Background(), "bead-cmt")
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/beads/bead-cmt/comments" {
		t.Errorf("path = %q, want /v1/beads/bead-cmt/comments", h.path)
	}

	if len(comments) != 2 {
		t.Fatalf("len(comments) = %d, want 2", len(comments))
	}
	if comments[0].Author != "alice" {
		t.Errorf("comments[0].Author = %q, want 'alice'", comments[0].Author)
	}
	if comments[1].Text != "Second" {
		t.Errorf("comments[1].Text = %q, want 'Second'", comments[1].Text)
	}
}

func TestHTTPClient_GetComments_Empty(t *testing.T) {
	h := &testHandler{
		responseBody: `{"comments": []}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	comments, err := c.GetComments(context.Background(), "bead-no-cmt")
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("len(comments) = %d, want 0", len(comments))
	}
}

// --- GetEvents ---

func TestHTTPClient_GetEvents(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"events": [
				{"id": 1, "topic": "bead.created", "bead_id": "bead-evt", "actor": "alice", "payload": {"title": "New"}, "created_at": "2026-01-15T10:00:00Z"},
				{"id": 2, "topic": "bead.updated", "bead_id": "bead-evt", "actor": "bob", "payload": {"status": "closed"}, "created_at": "2026-01-16T10:00:00Z"}
			]
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	events, err := c.GetEvents(context.Background(), "bead-evt")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/beads/bead-evt/events" {
		t.Errorf("path = %q, want /v1/beads/bead-evt/events", h.path)
	}

	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Topic != "bead.created" {
		t.Errorf("events[0].Topic = %q, want 'bead.created'", events[0].Topic)
	}
	if events[0].Actor != "alice" {
		t.Errorf("events[0].Actor = %q, want 'alice'", events[0].Actor)
	}
	if events[0].Payload == nil {
		t.Error("events[0].Payload is nil, want non-nil")
	}
	if events[1].ID != 2 {
		t.Errorf("events[1].ID = %d, want 2", events[1].ID)
	}
}

func TestHTTPClient_GetEvents_Empty(t *testing.T) {
	h := &testHandler{
		responseBody: `{"events": []}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	events, err := c.GetEvents(context.Background(), "bead-no-evt")
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("len(events) = %d, want 0", len(events))
	}
}

// --- SetConfig ---

func TestHTTPClient_SetConfig(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"key": "view:inbox",
			"value": {"columns": ["title", "status"]},
			"created_at": "2026-01-15T10:00:00Z",
			"updated_at": "2026-01-15T10:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	val := json.RawMessage(`{"columns": ["title", "status"]}`)
	cfg, err := c.SetConfig(context.Background(), "view:inbox", val)
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	if h.method != http.MethodPut {
		t.Errorf("method = %q, want PUT", h.method)
	}
	if h.path != "/v1/configs/view:inbox" {
		t.Errorf("path = %q, want /v1/configs/view:inbox", h.path)
	}

	var reqBody map[string]json.RawMessage
	if err := json.Unmarshal([]byte(h.body), &reqBody); err != nil {
		t.Fatalf("unmarshaling request body: %v", err)
	}
	// Compare structurally since json.Marshal may compact whitespace
	var gotVal, wantVal interface{}
	if err := json.Unmarshal(reqBody["value"], &gotVal); err != nil {
		t.Fatalf("unmarshaling request value: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"columns": ["title", "status"]}`), &wantVal); err != nil {
		t.Fatalf("unmarshaling expected value: %v", err)
	}
	gotJSON, _ := json.Marshal(gotVal)
	wantJSON, _ := json.Marshal(wantVal)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("request body value = %s, want %s", gotJSON, wantJSON)
	}

	if cfg.Key != "view:inbox" {
		t.Errorf("cfg.Key = %q, want 'view:inbox'", cfg.Key)
	}
	if cfg.Value == nil {
		t.Error("cfg.Value is nil, want non-nil")
	}
}

// --- GetConfig ---

func TestHTTPClient_GetConfig(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"key": "type:decision",
			"value": {"fields": [{"name": "status", "type": "enum"}]},
			"created_at": "2026-01-15T10:00:00Z",
			"updated_at": "2026-01-16T10:00:00Z"
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	cfg, err := c.GetConfig(context.Background(), "type:decision")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/configs/type:decision" {
		t.Errorf("path = %q, want /v1/configs/type:decision", h.path)
	}

	if cfg.Key != "type:decision" {
		t.Errorf("cfg.Key = %q, want 'type:decision'", cfg.Key)
	}
}

// --- ListConfigs ---

func TestHTTPClient_ListConfigs(t *testing.T) {
	h := &testHandler{
		responseBody: `{
			"configs": [
				{"key": "view:inbox", "value": {}, "created_at": "2026-01-15T10:00:00Z", "updated_at": "2026-01-15T10:00:00Z"},
				{"key": "view:board", "value": {}, "created_at": "2026-01-15T10:00:00Z", "updated_at": "2026-01-15T10:00:00Z"}
			]
		}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	configs, err := c.ListConfigs(context.Background(), "view")
	if err != nil {
		t.Fatalf("ListConfigs() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/configs" {
		t.Errorf("path = %q, want /v1/configs", h.path)
	}
	if !strings.Contains(h.query, "namespace=view") {
		t.Errorf("query %q missing namespace=view", h.query)
	}

	if len(configs) != 2 {
		t.Fatalf("len(configs) = %d, want 2", len(configs))
	}
	if configs[0].Key != "view:inbox" {
		t.Errorf("configs[0].Key = %q, want 'view:inbox'", configs[0].Key)
	}
}

func TestHTTPClient_ListConfigs_Empty(t *testing.T) {
	h := &testHandler{
		responseBody: `{"configs": []}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	configs, err := c.ListConfigs(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ListConfigs() error = %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("len(configs) = %d, want 0", len(configs))
	}
}

// --- DeleteConfig ---

func TestHTTPClient_DeleteConfig(t *testing.T) {
	h := &testHandler{
		statusCode: http.StatusNoContent,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.DeleteConfig(context.Background(), "view:inbox")
	if err != nil {
		t.Fatalf("DeleteConfig() error = %v", err)
	}

	if h.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", h.method)
	}
	if h.path != "/v1/configs/view:inbox" {
		t.Errorf("path = %q, want /v1/configs/view:inbox", h.path)
	}
}

// --- Health ---

func TestHTTPClient_Health(t *testing.T) {
	h := &testHandler{
		responseBody: `{"status": "ok"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	status, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	if h.method != http.MethodGet {
		t.Errorf("method = %q, want GET", h.method)
	}
	if h.path != "/v1/health" {
		t.Errorf("path = %q, want /v1/health", h.path)
	}

	if status != "ok" {
		t.Errorf("status = %q, want 'ok'", status)
	}
}

// --- Error handling ---

func TestHTTPClient_Error_JSONBody(t *testing.T) {
	h := &testHandler{
		statusCode:   http.StatusBadRequest,
		responseBody: `{"error": "bead title is required"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	_, err := c.CreateBead(context.Background(), &CreateBeadRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Message != "bead title is required" {
		t.Errorf("message = %q, want 'bead title is required'", apiErr.Message)
	}
}

func TestHTTPClient_Error_NonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)
	_, err := c.GetBead(context.Background(), "bead-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", apiErr.StatusCode)
	}
	if apiErr.Message != "internal server error" {
		t.Errorf("message = %q, want 'internal server error'", apiErr.Message)
	}
}

func TestHTTPClient_Error_404(t *testing.T) {
	h := &testHandler{
		statusCode:   http.StatusNotFound,
		responseBody: `{"error": "bead not found"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	_, err := c.GetBead(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "bead not found" {
		t.Errorf("message = %q, want 'bead not found'", apiErr.Message)
	}
}

func TestHTTPClient_Error_500(t *testing.T) {
	h := &testHandler{
		statusCode:   http.StatusInternalServerError,
		responseBody: `{"error": "database connection lost"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	err := c.DeleteBead(context.Background(), "bead-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", apiErr.StatusCode)
	}
}

func TestHTTPClient_Error_FormatString(t *testing.T) {
	apiErr := &APIError{StatusCode: 403, Message: "forbidden"}
	want := "HTTP 403: forbidden"
	if apiErr.Error() != want {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestHTTPClient_Error_EmptyJSONError(t *testing.T) {
	// JSON body with empty error field should use the raw body
	h := &testHandler{
		statusCode:   http.StatusUnprocessableEntity,
		responseBody: `{"error": ""}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	_, err := c.GetBead(context.Background(), "bead-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", apiErr.StatusCode)
	}
	// When errResp.Error is empty, the raw body is used as the message
	if apiErr.Message != `{"error": ""}` {
		t.Errorf("message = %q, want raw body", apiErr.Message)
	}
}

func TestHTTPClient_Error_CanceledContext(t *testing.T) {
	h := &testHandler{
		responseBody: `{"status": "ok"}`,
	}
	c, srv := newTestClient(h)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := c.Health(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	// The error should wrap context.Canceled
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error = %q, want to contain 'context canceled'", err.Error())
	}
}

// --- 204 No Content handling ---

func TestHTTPClient_204NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)

	// DeleteBead should succeed with 204
	err := c.DeleteBead(context.Background(), "bead-del")
	if err != nil {
		t.Fatalf("DeleteBead() with 204 error = %v", err)
	}

	// DeleteConfig should succeed with 204
	err = c.DeleteConfig(context.Background(), "some:key")
	if err != nil {
		t.Fatalf("DeleteConfig() with 204 error = %v", err)
	}

	// RemoveLabel should succeed with 204
	err = c.RemoveLabel(context.Background(), "bead-x", "label")
	if err != nil {
		t.Fatalf("RemoveLabel() with 204 error = %v", err)
	}

	// RemoveDependency should succeed with 204
	err = c.RemoveDependency(context.Background(), "bead-a", "bead-b", "blocks")
	if err != nil {
		t.Fatalf("RemoveDependency() with 204 error = %v", err)
	}
}

// --- Close ---

func TestHTTPClient_Close(t *testing.T) {
	c := NewHTTPClient("http://localhost:9999")
	if err := c.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

// --- NewHTTPClient base URL trimming ---

func TestNewHTTPClient_TrimsTrailingSlash(t *testing.T) {
	c := NewHTTPClient("http://localhost:8080/")
	if c.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want 'http://localhost:8080'", c.baseURL)
	}
}

func TestNewHTTPClient_NoTrailingSlash(t *testing.T) {
	c := NewHTTPClient("http://localhost:8080")
	if c.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want 'http://localhost:8080'", c.baseURL)
	}
}

// --- Interface compliance ---

func TestHTTPClient_ImplementsBeadsClient(t *testing.T) {
	var _ BeadsClient = (*HTTPClient)(nil)
}

// --- Concurrent requests ---

func TestHTTPClient_ConcurrentRequests(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL)

	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := c.Health(context.Background())
			errs <- err
		}()
	}

	for i := 0; i < 10; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent Health() error = %v", err)
		}
	}
}
