package store

import (
	"context"

	"github.com/groblegark/kbeads/internal/model"
)

// Store defines the persistence interface for beads.
type Store interface {
	// Bead CRUD
	CreateBead(ctx context.Context, bead *model.Bead) error
	GetBead(ctx context.Context, id string) (*model.Bead, error)
	ListBeads(ctx context.Context, filter model.BeadFilter) ([]*model.Bead, int, error) // returns beads, total count, error
	UpdateBead(ctx context.Context, bead *model.Bead) error
	CloseBead(ctx context.Context, id string, closedBy string) (*model.Bead, error)
	DeleteBead(ctx context.Context, id string) error

	// Dependencies
	AddDependency(ctx context.Context, dep *model.Dependency) error
	RemoveDependency(ctx context.Context, beadID, dependsOnID string, depType model.DependencyType) error
	GetDependencies(ctx context.Context, beadID string) ([]*model.Dependency, error)

	// Labels
	AddLabel(ctx context.Context, beadID string, label string) error
	RemoveLabel(ctx context.Context, beadID string, label string) error
	GetLabels(ctx context.Context, beadID string) ([]string, error)

	// Comments
	AddComment(ctx context.Context, comment *model.Comment) error
	GetComments(ctx context.Context, beadID string) ([]*model.Comment, error)

	// Events
	RecordEvent(ctx context.Context, event *model.Event) error
	GetEvents(ctx context.Context, beadID string) ([]*model.Event, error)

	// Configs
	SetConfig(ctx context.Context, config *model.Config) error
	GetConfig(ctx context.Context, key string) (*model.Config, error)
	ListConfigs(ctx context.Context, namespace string) ([]*model.Config, error)
	ListAllConfigs(ctx context.Context) ([]*model.Config, error)
	DeleteConfig(ctx context.Context, key string) error

	// Transaction support
	RunInTransaction(ctx context.Context, fn func(tx Store) error) error

	// Lifecycle
	Close() error
}
