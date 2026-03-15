package datastore

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/ports"
)

const kindJob = "Job"

// jobEntity is the Datastore entity shape for job index.
type jobEntity struct {
	OwnerSub   string    `datastore:"ownerSub"`
	OwnerEmail string    `datastore:"ownerEmail"`
	Title      string    `datastore:"title"`
	CreatedAt  time.Time `datastore:"createdAt"`
}

// Index implements ports.JobIndex using Cloud Datastore (or Firestore in Datastore mode).
type Index struct {
	client *datastore.Client
}

// NewIndex creates a Datastore job index. projectID must be non-empty.
// databaseID is the named database (e.g. "code-commenter"); empty means "(default)".
func NewIndex(ctx context.Context, projectID, databaseID string) (*Index, error) {
	if projectID == "" {
		return nil, nil
	}
	var client *datastore.Client
	var err error
	if databaseID == "" {
		client, err = datastore.NewClient(ctx, projectID)
	} else {
		client, err = datastore.NewClientWithDatabase(ctx, projectID, databaseID)
	}
	if err != nil {
		return nil, err
	}
	return &Index{client: client}, nil
}

// Close releases the Datastore client.
func (i *Index) Close() error {
	if i == nil || i.client == nil {
		return nil
	}
	return i.client.Close()
}

// Add writes job metadata to Datastore (key = kind "Job", name = jobID).
func (i *Index) Add(ctx context.Context, jobID, ownerSub, ownerEmail, title string) error {
	if i == nil || i.client == nil {
		return nil
	}
	key := datastore.NameKey(kindJob, jobID, nil)
	_, err := i.client.Put(ctx, key, &jobEntity{
		OwnerSub:   ownerSub,
		OwnerEmail: ownerEmail,
		Title:      title,
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		log.Error().Err(err).Str("job", jobID).Msg("datastore index add")
		return err
	}
	return nil
}

// ListRecent returns the most recently created jobs across all owners, newest first.
// Uses a single-property descending index on createdAt (auto-created by Datastore).
func (i *Index) ListRecent(ctx context.Context, limit int) ([]ports.JobMeta, error) {
	if i == nil || i.client == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	q := datastore.NewQuery(kindJob).
		Order("-createdAt").
		Limit(limit)
	var entities []jobEntity
	keys, err := i.client.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, err
	}
	out := make([]ports.JobMeta, 0, len(keys))
	for idx, k := range keys {
		if k.Name == "" {
			continue
		}
		ent := entities[idx]
		out = append(out, ports.JobMeta{
			ID:        k.Name,
			Title:     ent.Title,
			CreatedAt: ent.CreatedAt.UnixMilli(),
		})
	}
	return out, nil
}

// ListByOwner returns jobs for the given owner, newest first.
// Requires a composite index: kind "Job", ownerSub (Ascending), createdAt (Descending).
// See api/index.yaml or README.
func (i *Index) ListByOwner(ctx context.Context, ownerSub string, limit int) ([]ports.JobMeta, error) {
	if i == nil || i.client == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	q := datastore.NewQuery(kindJob).
		Filter("ownerSub =", ownerSub).
		Order("-createdAt").
		Limit(limit)
	var entities []jobEntity
	keys, err := i.client.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, err
	}
	out := make([]ports.JobMeta, 0, len(keys))
	for idx, k := range keys {
		if k.Name == "" {
			continue
		}
		ent := entities[idx]
		out = append(out, ports.JobMeta{
			ID:        k.Name,
			Title:     ent.Title,
			CreatedAt: ent.CreatedAt.UnixMilli(),
		})
	}
	return out, nil
}
