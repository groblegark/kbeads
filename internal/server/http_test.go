package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/groblegark/kbeads/internal/events"
	"github.com/groblegark/kbeads/internal/model"
	"github.com/groblegark/kbeads/internal/store"
)

type mockStore struct {
	beads         map[string]*model.Bead
	configs       map[string]*model.Config
	events        []*model.Event
	deps          map[string][]*model.Dependency
	labels        map[string][]string
	comments      map[string][]*model.Comment
	commentNextID int64

	// addLabelErr, when non-nil, is returned by AddLabel (for testing rollback).
	addLabelErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		beads:    make(map[string]*model.Bead),
		configs:  make(map[string]*model.Config),
		deps:     make(map[string][]*model.Dependency),
		labels:   make(map[string][]string),
		comments: make(map[string][]*model.Comment),
	}
}

func (m *mockStore) CreateBead(_ context.Context, bead *model.Bead) error {
	m.beads[bead.ID] = bead
	return nil
}

func (m *mockStore) GetBead(_ context.Context, id string) (*model.Bead, error) {
	b, ok := m.beads[id]
	if !ok {
		return nil, nil
	}
	// Clone and attach labels so callers see the latest label state.
	clone := *b
	clone.Labels = m.labels[id]
	return &clone, nil
}

func (m *mockStore) ListBeads(_ context.Context, filter model.BeadFilter) ([]*model.Bead, int, error) {
	var result []*model.Bead
outer:
	for _, b := range m.beads {
		if len(filter.Status) > 0 {
			found := false
			for _, s := range filter.Status {
				if b.Status == s {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if len(filter.Type) > 0 {
			found := false
			for _, t := range filter.Type {
				if b.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if len(filter.Kind) > 0 {
			found := false
			for _, k := range filter.Kind {
				if b.Kind == k {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if filter.Priority != nil && b.Priority != *filter.Priority {
			continue
		}
		if filter.Assignee != "" && b.Assignee != filter.Assignee {
			continue
		}
		if len(filter.Labels) > 0 {
			beadLabels := m.labels[b.ID]
			for _, want := range filter.Labels {
				found := false
				for _, have := range beadLabels {
					if have == want {
						found = true
						break
					}
				}
				if !found {
					continue outer
				}
			}
		}
		if filter.Search != "" {
			if !strings.Contains(strings.ToLower(b.Title), strings.ToLower(filter.Search)) &&
				!strings.Contains(strings.ToLower(b.Description), strings.ToLower(filter.Search)) {
				continue
			}
		}
		result = append(result, b)
	}
	return result, len(result), nil
}

func (m *mockStore) UpdateBead(_ context.Context, bead *model.Bead) error {
	m.beads[bead.ID] = bead
	return nil
}

func (m *mockStore) CloseBead(_ context.Context, id string, closedBy string) (*model.Bead, error) {
	b, ok := m.beads[id]
	if !ok {
		return nil, nil
	}
	now := time.Now().UTC()
	b.Status = model.StatusClosed
	b.ClosedAt = &now
	b.ClosedBy = closedBy
	b.UpdatedAt = now
	return b, nil
}

func (m *mockStore) DeleteBead(_ context.Context, id string) error {
	if _, ok := m.beads[id]; !ok {
		return sql.ErrNoRows
	}
	delete(m.beads, id)
	delete(m.labels, id)
	return nil
}

func (m *mockStore) AddDependency(_ context.Context, dep *model.Dependency) error {
	m.deps[dep.BeadID] = append(m.deps[dep.BeadID], dep)
	return nil
}

func (m *mockStore) RemoveDependency(_ context.Context, beadID, dependsOnID string, depType model.DependencyType) error {
	deps := m.deps[beadID]
	for i, d := range deps {
		if d.DependsOnID == dependsOnID && d.Type == depType {
			m.deps[beadID] = append(deps[:i], deps[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockStore) GetDependencies(_ context.Context, beadID string) ([]*model.Dependency, error) {
	return m.deps[beadID], nil
}

func (m *mockStore) AddLabel(_ context.Context, beadID string, label string) error {
	if m.addLabelErr != nil {
		return m.addLabelErr
	}
	// Skip duplicates (mirrors ON CONFLICT DO NOTHING).
	for _, l := range m.labels[beadID] {
		if l == label {
			return nil
		}
	}
	m.labels[beadID] = append(m.labels[beadID], label)
	return nil
}

func (m *mockStore) RemoveLabel(_ context.Context, beadID string, label string) error {
	labels := m.labels[beadID]
	for i, l := range labels {
		if l == label {
			m.labels[beadID] = append(labels[:i], labels[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockStore) GetLabels(_ context.Context, beadID string) ([]string, error) {
	return m.labels[beadID], nil
}

func (m *mockStore) AddComment(_ context.Context, comment *model.Comment) error {
	m.commentNextID++
	comment.ID = m.commentNextID
	m.comments[comment.BeadID] = append(m.comments[comment.BeadID], comment)
	return nil
}

func (m *mockStore) GetComments(_ context.Context, beadID string) ([]*model.Comment, error) {
	return m.comments[beadID], nil
}

func (m *mockStore) RecordEvent(_ context.Context, event *model.Event) error {
	event.ID = int64(len(m.events) + 1)
	m.events = append(m.events, event)
	return nil
}

func (m *mockStore) GetEvents(_ context.Context, beadID string) ([]*model.Event, error) {
	var result []*model.Event
	for _, e := range m.events {
		if e.BeadID == beadID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockStore) SetConfig(_ context.Context, config *model.Config) error {
	m.configs[config.Key] = config
	return nil
}

func (m *mockStore) GetConfig(_ context.Context, key string) (*model.Config, error) {
	c, ok := m.configs[key]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return c, nil
}

func (m *mockStore) ListConfigs(_ context.Context, namespace string) ([]*model.Config, error) {
	prefix := namespace + ":"
	var result []*model.Config
	for k, c := range m.configs {
		if strings.HasPrefix(k, prefix) {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockStore) ListAllConfigs(_ context.Context) ([]*model.Config, error) {
	var result []*model.Config
	for _, c := range m.configs {
		result = append(result, c)
	}
	return result, nil
}

func (m *mockStore) DeleteConfig(_ context.Context, key string) error {
	if _, ok := m.configs[key]; !ok {
		return sql.ErrNoRows
	}
	delete(m.configs, key)
	return nil
}

func (m *mockStore) RunInTransaction(_ context.Context, fn func(tx store.Store) error) error {
	return fn(m)
}

func (m *mockStore) Close() error {
	return nil
}

// newTestServer returns a fresh server, its mock store, and an HTTP handler.
func newTestServer() (*BeadsServer, *mockStore, http.Handler) {
	ms := newMockStore()
	s := NewBeadsServer(ms, &events.NoopPublisher{})
	return s, ms, s.NewHTTPHandler()
}

// doJSON performs an HTTP request with an optional JSON body and returns the recorder.
func doJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// requireStatus asserts the recorder has the expected HTTP status code.
func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("expected status %d, got %d; body: %s", code, rec.Code, rec.Body.String())
	}
}

// decodeJSON decodes the recorder's response body into v.
func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestHandleHTTPErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		method    string
		path      string
		body      any
		code      int
		wantError string
	}{
		{"CreateBead/MissingTitle", "POST", "/v1/beads", map[string]any{"type": "task"}, 400, "title is required"},
		{"GetBead/NotFound", "GET", "/v1/beads/nonexistent", nil, 404, "bead not found"},
		{"DeleteBead/NotFound", "DELETE", "/v1/beads/nonexistent", nil, 404, ""},
		{"GetConfig/NotFound", "GET", "/v1/configs/view:nonexistent", nil, 404, ""},
		{"DeleteConfig/NotFound", "DELETE", "/v1/configs/view:nonexistent", nil, 404, ""},
		{"AddComment/MissingText", "POST", "/v1/beads/kd-x/comments", map[string]any{"author": "alice"}, 400, ""},
		{"AddDependency/MissingDependsOnID", "POST", "/v1/beads/kd-a/dependencies", map[string]any{"type": "blocks"}, 400, ""},
		{"AddLabel/MissingLabel", "POST", "/v1/beads/kd-a/labels", map[string]any{}, 400, ""},
		{"RemoveDependency/MissingDependsOnID", "DELETE", "/v1/beads/kd-a/dependencies?type=blocks", nil, 400, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, h := newTestServer()
			rec := doJSON(t, h, tc.method, tc.path, tc.body)
			requireStatus(t, rec, tc.code)
			if tc.wantError != "" {
				var body map[string]string
				decodeJSON(t, rec, &body)
				if body["error"] != tc.wantError {
					t.Fatalf("expected error=%q, got %q", tc.wantError, body["error"])
				}
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "GET", "/v1/health", nil)
	requireStatus(t, rec, 200)
	var body map[string]string
	decodeJSON(t, rec, &body)
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}

func TestHandleCreateBead(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{"title": "Test bead", "type": "task"})
	requireStatus(t, rec, 201)
	var bead model.Bead
	decodeJSON(t, rec, &bead)
	if bead.ID == "" {
		t.Fatal("expected bead to have an ID")
	}
	if bead.Title != "Test bead" || bead.Status != model.StatusOpen || bead.Kind != model.KindIssue {
		t.Fatalf("got title=%q status=%q kind=%q", bead.Title, bead.Status, bead.Kind)
	}
}

func TestHandleListBeads(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-abc123"] = &model.Bead{ID: "kd-abc123", Title: "Bead one", Status: model.StatusOpen}
	ms.beads["kd-def456"] = &model.Bead{ID: "kd-def456", Title: "Bead two", Status: model.StatusOpen}

	rec := doJSON(t, h, "GET", "/v1/beads", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Beads []model.Bead `json:"beads"`
		Total int          `json:"total"`
	}
	decodeJSON(t, rec, &result)
	if result.Total != 2 || len(result.Beads) != 2 {
		t.Fatalf("expected 2 beads, got total=%d len=%d", result.Total, len(result.Beads))
	}
}

func TestHandleListBeads_WithFilters(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-f1"] = &model.Bead{ID: "kd-f1", Title: "Open test bead", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen, Assignee: "alice"}
	ms.beads["kd-f2"] = &model.Bead{ID: "kd-f2", Title: "Closed bead", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusClosed, Assignee: "alice"}

	rec := doJSON(t, h, "GET", "/v1/beads?status=open&type=task&kind=issue&limit=10&offset=0&sort=-priority&assignee=alice&search=test", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Beads []model.Bead `json:"beads"`
		Total int          `json:"total"`
	}
	decodeJSON(t, rec, &result)
	if result.Total != 1 {
		t.Fatalf("expected total=1 (only open matching), got %d", result.Total)
	}
}

func TestHandleGetBead(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-test1"] = &model.Bead{ID: "kd-test1", Title: "Test bead", Status: model.StatusOpen}

	rec := doJSON(t, h, "GET", "/v1/beads/kd-test1", nil)
	requireStatus(t, rec, 200)
	var bead model.Bead
	decodeJSON(t, rec, &bead)
	if bead.ID != "kd-test1" || bead.Title != "Test bead" {
		t.Fatalf("got id=%q title=%q", bead.ID, bead.Title)
	}
}

func TestHandleSetConfig(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "PUT", "/v1/configs/view:inbox", map[string]any{
		"value": map[string]any{"filter": map[string]any{"status": []string{"open"}}},
	})
	requireStatus(t, rec, 200)
	var config model.Config
	decodeJSON(t, rec, &config)
	if config.Key != "view:inbox" {
		t.Fatalf("expected key=%q, got %q", "view:inbox", config.Key)
	}
}

func TestHandleGetConfig(t *testing.T) {
	_, ms, h := newTestServer()
	ms.configs["view:inbox"] = &model.Config{Key: "view:inbox", Value: json.RawMessage(`{"filter":{"status":["open"]}}`)}

	rec := doJSON(t, h, "GET", "/v1/configs/view:inbox", nil)
	requireStatus(t, rec, 200)
	var config model.Config
	decodeJSON(t, rec, &config)
	if config.Key != "view:inbox" {
		t.Fatalf("expected key=%q, got %q", "view:inbox", config.Key)
	}
}

func TestHandleListConfigs(t *testing.T) {
	_, ms, h := newTestServer()
	ms.configs["view:inbox"] = &model.Config{Key: "view:inbox", Value: json.RawMessage(`{}`)}
	ms.configs["view:my-tasks"] = &model.Config{Key: "view:my-tasks", Value: json.RawMessage(`{}`)}
	ms.configs["type:decision"] = &model.Config{Key: "type:decision", Value: json.RawMessage(`{}`)}

	rec := doJSON(t, h, "GET", "/v1/configs?namespace=view", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Configs []model.Config `json:"configs"`
	}
	decodeJSON(t, rec, &result)
	// 2 stored + 1 builtin (view:ready)
	if len(result.Configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(result.Configs))
	}
}

func TestHandleDeleteConfig(t *testing.T) {
	_, ms, h := newTestServer()
	ms.configs["view:inbox"] = &model.Config{Key: "view:inbox", Value: json.RawMessage(`{}`)}

	rec := doJSON(t, h, "DELETE", "/v1/configs/view:inbox", nil)
	requireStatus(t, rec, 204)
	if _, ok := ms.configs["view:inbox"]; ok {
		t.Fatal("expected config to be deleted")
	}
}

func TestHandleGetConfig_BuiltinFallback(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "GET", "/v1/configs/view:ready", nil)
	requireStatus(t, rec, 200)
	var config model.Config
	decodeJSON(t, rec, &config)
	if config.Key != "view:ready" {
		t.Fatalf("expected key=%q, got %q", "view:ready", config.Key)
	}
}

func TestHandleGetConfig_StoreOverridesBuiltin(t *testing.T) {
	_, ms, h := newTestServer()
	ms.configs["view:ready"] = &model.Config{Key: "view:ready", Value: json.RawMessage(`{"limit":99}`)}

	rec := doJSON(t, h, "GET", "/v1/configs/view:ready", nil)
	requireStatus(t, rec, 200)
	var config model.Config
	decodeJSON(t, rec, &config)
	if string(config.Value) != `{"limit":99}` {
		t.Fatalf("expected store override, got %s", config.Value)
	}
}

func TestHandleListConfigs_IncludesBuiltins(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "GET", "/v1/configs?namespace=view", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Configs []model.Config `json:"configs"`
	}
	decodeJSON(t, rec, &result)
	if len(result.Configs) != 1 || result.Configs[0].Key != "view:ready" {
		t.Fatalf("expected 1 builtin config (view:ready), got %d", len(result.Configs))
	}
}

func TestHandleListConfigs_BuiltinNotDuplicated(t *testing.T) {
	_, ms, h := newTestServer()
	ms.configs["view:ready"] = &model.Config{Key: "view:ready", Value: json.RawMessage(`{"limit":99}`)}

	rec := doJSON(t, h, "GET", "/v1/configs?namespace=view", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Configs []model.Config `json:"configs"`
	}
	decodeJSON(t, rec, &result)
	if len(result.Configs) != 1 {
		t.Fatalf("expected 1 config (no duplicate), got %d", len(result.Configs))
	}
	if string(result.Configs[0].Value) != `{"limit":99}` {
		t.Fatalf("expected store override value, got %s", result.Configs[0].Value)
	}
}

func TestHandleGetDependencies(t *testing.T) {
	_, ms, h := newTestServer()
	ms.deps["kd-dep1"] = []*model.Dependency{
		{BeadID: "kd-dep1", DependsOnID: "kd-dep2", Type: "blocks"},
		{BeadID: "kd-dep1", DependsOnID: "kd-dep3", Type: "related"},
	}
	rec := doJSON(t, h, "GET", "/v1/beads/kd-dep1/dependencies", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Dependencies []model.Dependency `json:"dependencies"`
	}
	decodeJSON(t, rec, &result)
	if len(result.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(result.Dependencies))
	}
}

func TestHandleGetLabels(t *testing.T) {
	_, ms, h := newTestServer()
	ms.labels["kd-lbl1"] = []string{"urgent", "frontend"}

	rec := doJSON(t, h, "GET", "/v1/beads/kd-lbl1/labels", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Labels []string `json:"labels"`
	}
	decodeJSON(t, rec, &result)
	if len(result.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(result.Labels))
	}
}

func TestHandleGetComments(t *testing.T) {
	_, ms, h := newTestServer()
	ms.comments["kd-cmt1"] = []*model.Comment{
		{ID: 1, BeadID: "kd-cmt1", Author: "alice", Text: "First"},
		{ID: 2, BeadID: "kd-cmt1", Author: "bob", Text: "Second"},
	}
	rec := doJSON(t, h, "GET", "/v1/beads/kd-cmt1/comments", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Comments []model.Comment `json:"comments"`
	}
	decodeJSON(t, rec, &result)
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}
}

func TestHandleGetEvents(t *testing.T) {
	_, ms, h := newTestServer()
	ms.events = []*model.Event{
		{ID: 1, Topic: "beads.bead.created", BeadID: "kd-abc123", Actor: "alice", Payload: json.RawMessage(`{}`)},
		{ID: 2, Topic: "beads.label.added", BeadID: "kd-abc123", Payload: json.RawMessage(`{}`)},
		{ID: 3, Topic: "beads.bead.created", BeadID: "kd-other", Payload: json.RawMessage(`{}`)},
	}
	rec := doJSON(t, h, "GET", "/v1/beads/kd-abc123/events", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Events []model.Event `json:"events"`
	}
	decodeJSON(t, rec, &result)
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}

func TestHandleEmptyLists(t *testing.T) {
	_, _, h := newTestServer()
	for _, tc := range []struct {
		name string
		path string
	}{
		{"Dependencies", "/v1/beads/kd-nope/dependencies"},
		{"Labels", "/v1/beads/kd-nope/labels"},
		{"Comments", "/v1/beads/kd-nope/comments"},
		{"Events", "/v1/beads/kd-nonexistent/events"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, h, "GET", tc.path, nil)
			requireStatus(t, rec, 200)
		})
	}
}

func TestHandleAddComment_Response(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-cmt2"] = &model.Bead{ID: "kd-cmt2", Title: "Bead for comment", Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-cmt2/comments", map[string]any{"author": "alice", "text": "A new comment"})
	requireStatus(t, rec, 201)
	var comment model.Comment
	decodeJSON(t, rec, &comment)
	if comment.Text != "A new comment" || comment.Author != "alice" || comment.BeadID != "kd-cmt2" {
		t.Fatalf("got text=%q author=%q bead_id=%q", comment.Text, comment.Author, comment.BeadID)
	}
}

func TestCreateBeadRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{"title": "Test bead", "type": "task", "created_by": "alice"})
	requireStatus(t, rec, 201)
	requireEvent(t, ms, 1, "beads.bead.created")
	if ms.events[0].Actor != "alice" {
		t.Fatalf("expected actor=%q, got %q", "alice", ms.events[0].Actor)
	}
}

func TestDeleteBeadRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-del1"] = &model.Bead{ID: "kd-del1", Title: "To delete", Status: model.StatusOpen}

	rec := doJSON(t, h, "DELETE", "/v1/beads/kd-del1", nil)
	requireStatus(t, rec, 204)
	requireEvent(t, ms, 1, "beads.bead.deleted")
	if ms.events[0].BeadID != "kd-del1" {
		t.Fatalf("expected bead_id=%q, got %q", "kd-del1", ms.events[0].BeadID)
	}
}

func TestAddCommentRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-cmt1"] = &model.Bead{ID: "kd-cmt1", Title: "Bead with comment", Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-cmt1/comments", map[string]any{"author": "bob", "text": "Hello world"})
	requireStatus(t, rec, 201)
	requireEvent(t, ms, 1, "beads.comment.added")
	if ms.events[0].Actor != "bob" {
		t.Fatalf("expected actor=%q, got %q", "bob", ms.events[0].Actor)
	}
}

func TestAddLabelRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-lbl1"] = &model.Bead{ID: "kd-lbl1", Title: "Bead with label", Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-lbl1/labels", map[string]any{"label": "urgent"})
	requireStatus(t, rec, 201)
	requireEvent(t, ms, 1, "beads.label.added")
}

func TestRemoveLabelRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-lbl2"] = &model.Bead{ID: "kd-lbl2", Title: "Bead losing label", Status: model.StatusOpen}

	rec := doJSON(t, h, "DELETE", "/v1/beads/kd-lbl2/labels/urgent", nil)
	requireStatus(t, rec, 204)
	requireEvent(t, ms, 1, "beads.label.removed")
	var labelEvt events.LabelRemoved
	if err := json.Unmarshal(ms.events[0].Payload, &labelEvt); err != nil {
		t.Fatalf("failed to unmarshal event payload: %v", err)
	}
	if labelEvt.Label != "urgent" {
		t.Fatalf("expected label=%q in payload, got %q", "urgent", labelEvt.Label)
	}
}

func TestAddDependencyRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-dep1"] = &model.Bead{ID: "kd-dep1", Title: "A", Status: model.StatusOpen}
	ms.beads["kd-dep2"] = &model.Bead{ID: "kd-dep2", Title: "B", Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-dep1/dependencies", map[string]any{
		"depends_on_id": "kd-dep2", "type": "blocks", "created_by": "carol",
	})
	requireStatus(t, rec, 201)
	requireEvent(t, ms, 1, "beads.dependency.added")
	if ms.events[0].Actor != "carol" {
		t.Fatalf("expected actor=%q, got %q", "carol", ms.events[0].Actor)
	}
}

func TestRemoveDependencyRecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-rdep1"] = &model.Bead{ID: "kd-rdep1", Title: "A", Status: model.StatusOpen}

	rec := doJSON(t, h, "DELETE", "/v1/beads/kd-rdep1/dependencies?depends_on_id=kd-rdep2&type=blocks", nil)
	requireStatus(t, rec, 204)
	requireEvent(t, ms, 1, "beads.dependency.removed")
	var depEvt events.DependencyRemoved
	if err := json.Unmarshal(ms.events[0].Payload, &depEvt); err != nil {
		t.Fatalf("failed to unmarshal event payload: %v", err)
	}
	if depEvt.DependsOnID != "kd-rdep2" || depEvt.Type != "blocks" {
		t.Fatalf("expected depends_on_id=%q type=%q, got %q %q", "kd-rdep2", "blocks", depEvt.DependsOnID, depEvt.Type)
	}
}

func TestHandleUpdateBead(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-upd1"] = &model.Bead{ID: "kd-upd1", Title: "Original", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}

	rec := doJSON(t, h, "PATCH", "/v1/beads/kd-upd1", map[string]any{"title": "Updated"})
	requireStatus(t, rec, 200)
	var bead model.Bead
	decodeJSON(t, rec, &bead)
	if bead.Title != "Updated" {
		t.Fatalf("expected title=%q, got %q", "Updated", bead.Title)
	}
}

func TestHandleUpdateBead_LabelsReconciled(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-upd2"] = &model.Bead{ID: "kd-upd2", Title: "Bead", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}
	ms.labels["kd-upd2"] = []string{"a", "b"}

	rec := doJSON(t, h, "PATCH", "/v1/beads/kd-upd2", map[string]any{"labels": []string{"b", "c"}})
	requireStatus(t, rec, 200)

	// Verify via GET that labels were reconciled in the store.
	rec = doJSON(t, h, "GET", "/v1/beads/kd-upd2", nil)
	requireStatus(t, rec, 200)
	var bead model.Bead
	decodeJSON(t, rec, &bead)
	if len(bead.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(bead.Labels), bead.Labels)
	}
	labelSet := map[string]bool{}
	for _, l := range bead.Labels {
		labelSet[l] = true
	}
	if !labelSet["b"] || !labelSet["c"] {
		t.Fatalf("expected labels [b, c], got %v", bead.Labels)
	}
}

func TestHandleUpdateBead_NotFound(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "PATCH", "/v1/beads/nonexistent", map[string]any{"title": "x"})
	requireStatus(t, rec, 404)
}

func TestHandleUpdateBead_InvalidJSON(t *testing.T) {
	_, _, h := newTestServer()
	req := httptest.NewRequest("PATCH", "/v1/beads/kd-x", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireStatus(t, rec, 400)
}

func TestHandleCloseBead(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-cls1"] = &model.Bead{ID: "kd-cls1", Title: "To close", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-cls1/close", nil)
	requireStatus(t, rec, 200)
	var bead model.Bead
	decodeJSON(t, rec, &bead)
	if bead.Status != model.StatusClosed {
		t.Fatalf("expected status=%q, got %q", model.StatusClosed, bead.Status)
	}
}

func TestHandleCloseBead_WithClosedBy(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-cls2"] = &model.Bead{ID: "kd-cls2", Title: "To close", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-cls2/close", map[string]any{"closed_by": "alice"})
	requireStatus(t, rec, 200)
	var bead model.Bead
	decodeJSON(t, rec, &bead)
	if bead.ClosedBy != "alice" {
		t.Fatalf("expected closed_by=%q, got %q", "alice", bead.ClosedBy)
	}
}

func TestHandleCloseBead_RecordsEvent(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-cls3"] = &model.Bead{ID: "kd-cls3", Title: "To close", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}

	rec := doJSON(t, h, "POST", "/v1/beads/kd-cls3/close", map[string]any{"closed_by": "bob"})
	requireStatus(t, rec, 200)
	requireEvent(t, ms, 1, "beads.bead.closed")
	if ms.events[0].Actor != "bob" {
		t.Fatalf("expected actor=%q, got %q", "bob", ms.events[0].Actor)
	}
}

func TestHandleCloseBead_NotFound(t *testing.T) {
	_, _, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads/nonexistent/close", nil)
	requireStatus(t, rec, 404)
}

func TestHandleListBeads_FilterByLabels(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-l1"] = &model.Bead{ID: "kd-l1", Title: "Has urgent", Status: model.StatusOpen}
	ms.labels["kd-l1"] = []string{"urgent"}
	ms.beads["kd-l2"] = &model.Bead{ID: "kd-l2", Title: "Has frontend", Status: model.StatusOpen}
	ms.labels["kd-l2"] = []string{"frontend"}

	rec := doJSON(t, h, "GET", "/v1/beads?labels=urgent", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Beads []model.Bead `json:"beads"`
		Total int          `json:"total"`
	}
	decodeJSON(t, rec, &result)
	if result.Total != 1 {
		t.Fatalf("expected total=1, got %d", result.Total)
	}
	if result.Beads[0].ID != "kd-l1" {
		t.Fatalf("expected bead kd-l1, got %q", result.Beads[0].ID)
	}
}

func TestHandleListBeads_FilterByPriority(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-p1"] = &model.Bead{ID: "kd-p1", Title: "High", Status: model.StatusOpen, Priority: 3}
	ms.beads["kd-p2"] = &model.Bead{ID: "kd-p2", Title: "Low", Status: model.StatusOpen, Priority: 1}

	rec := doJSON(t, h, "GET", "/v1/beads?priority=3", nil)
	requireStatus(t, rec, 200)
	var result struct {
		Beads []model.Bead `json:"beads"`
		Total int          `json:"total"`
	}
	decodeJSON(t, rec, &result)
	if result.Total != 1 {
		t.Fatalf("expected total=1, got %d", result.Total)
	}
	if result.Beads[0].ID != "kd-p1" {
		t.Fatalf("expected bead kd-p1, got %q", result.Beads[0].ID)
	}
}

func TestHandleCreateBead_LabelFailure_ReturnsError(t *testing.T) {
	_, ms, h := newTestServer()
	ms.addLabelErr = fmt.Errorf("label store down")

	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "With labels", "type": "task", "labels": []string{"x"},
	})
	requireStatus(t, rec, 500)
}

func TestHandleCreateBead_WithLabels_AllPersisted(t *testing.T) {
	_, ms, h := newTestServer()
	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "With labels", "type": "task", "labels": []string{"a", "b", "c"},
	})
	requireStatus(t, rec, 201)
	var bead model.Bead
	decodeJSON(t, rec, &bead)

	// Verify all 3 labels are in the store.
	if len(ms.labels[bead.ID]) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(ms.labels[bead.ID]))
	}

	// Verify GET returns them.
	rec = doJSON(t, h, "GET", "/v1/beads/"+bead.ID, nil)
	requireStatus(t, rec, 200)
	var got model.Bead
	decodeJSON(t, rec, &got)
	if len(got.Labels) != 3 {
		t.Fatalf("expected 3 labels on GET, got %d", len(got.Labels))
	}
}

func TestHandleUpdateBead_LabelsPreservedWhenNotSpecified(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-upd3"] = &model.Bead{ID: "kd-upd3", Title: "Original", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}
	ms.labels["kd-upd3"] = []string{"keep-me"}

	// Update only title, don't send labels.
	rec := doJSON(t, h, "PATCH", "/v1/beads/kd-upd3", map[string]any{"title": "New title"})
	requireStatus(t, rec, 200)

	// Labels should be unchanged.
	if len(ms.labels["kd-upd3"]) != 1 || ms.labels["kd-upd3"][0] != "keep-me" {
		t.Fatalf("expected labels to be preserved, got %v", ms.labels["kd-upd3"])
	}
}

func TestHandleUpdateBead_ClearLabels(t *testing.T) {
	_, ms, h := newTestServer()
	ms.beads["kd-upd4"] = &model.Bead{ID: "kd-upd4", Title: "Has labels", Kind: model.KindIssue, Type: model.TypeTask, Status: model.StatusOpen}
	ms.labels["kd-upd4"] = []string{"a", "b"}

	// Update with empty labels array to clear them.
	rec := doJSON(t, h, "PATCH", "/v1/beads/kd-upd4", map[string]any{"labels": []string{}})
	requireStatus(t, rec, 200)

	if len(ms.labels["kd-upd4"]) != 0 {
		t.Fatalf("expected 0 labels, got %d: %v", len(ms.labels["kd-upd4"]), ms.labels["kd-upd4"])
	}
}

func TestHandleCreateBead_DueAtOnNonIssue(t *testing.T) {
	_, ms, h := newTestServer()
	ms.configs["type:note"] = &model.Config{Key: "type:note", Value: json.RawMessage(`{"kind":"data"}`)}

	rec := doJSON(t, h, "POST", "/v1/beads", map[string]any{
		"title": "x", "type": "note", "due_at": "2026-03-01T00:00:00Z",
	})
	requireStatus(t, rec, 201)
}
