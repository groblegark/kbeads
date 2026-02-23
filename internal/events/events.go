package events

import (
	"context"

	"github.com/groblegark/kbeads/internal/model"
)

// Event topic constants
const (
	TopicBeadCreated       = "beads.bead.created"
	TopicBeadUpdated       = "beads.bead.updated"
	TopicBeadClosed        = "beads.bead.closed"
	TopicBeadDeleted       = "beads.bead.deleted"
	TopicDependencyAdded   = "beads.dependency.added"
	TopicDependencyRemoved = "beads.dependency.removed"
	TopicLabelAdded        = "beads.label.added"
	TopicLabelRemoved      = "beads.label.removed"
	TopicCommentAdded      = "beads.comment.added"
)

// Event types

type BeadCreated struct {
	Bead *model.Bead `json:"bead"`
}

type BeadUpdated struct {
	Bead    *model.Bead    `json:"bead"`
	Changes map[string]any `json:"changes"` // field name -> new value
}

type BeadClosed struct {
	Bead     *model.Bead `json:"bead"`
	ClosedBy string      `json:"closed_by,omitempty"`
}

type BeadDeleted struct {
	BeadID string `json:"bead_id"`
}

type DependencyAdded struct {
	Dependency *model.Dependency `json:"dependency"`
}

type DependencyRemoved struct {
	BeadID      string `json:"bead_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

type LabelAdded struct {
	BeadID string `json:"bead_id"`
	Label  string `json:"label"`
}

type LabelRemoved struct {
	BeadID string `json:"bead_id"`
	Label  string `json:"label"`
}

type CommentAdded struct {
	Comment *model.Comment `json:"comment"`
}

// Publisher is the interface for emitting events.
type Publisher interface {
	Publish(ctx context.Context, topic string, event any) error
	Close() error
}
