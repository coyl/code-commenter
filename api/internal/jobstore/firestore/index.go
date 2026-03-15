package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/ports"
)

const collectionJobs = "jobs"

// jobDoc is the Firestore document shape for job index.
type jobDoc struct {
	OwnerSub   string    `firestore:"ownerSub"`
	OwnerEmail string    `firestore:"ownerEmail"`
	Title      string    `firestore:"title"`
	CreatedAt  time.Time `firestore:"createdAt"`
}

// Index implements ports.JobIndex using Firestore.
type Index struct {
	client *firestore.Client
}

// NewIndex creates a Firestore job index. projectID must be non-empty.
// databaseID is the Firestore database name; empty means the default database "(default)".
func NewIndex(ctx context.Context, projectID, databaseID string) (*Index, error) {
	if projectID == "" {
		return nil, nil
	}
	var client *firestore.Client
	var err error
	if databaseID == "" {
		client, err = firestore.NewClient(ctx, projectID)
	} else {
		client, err = firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	}
	if err != nil {
		return nil, err
	}
	return &Index{client: client}, nil
}

// Close releases the Firestore client.
func (i *Index) Close() error {
	if i == nil || i.client == nil {
		return nil
	}
	return i.client.Close()
}

// Add writes job metadata to Firestore (document ID = jobID).
func (i *Index) Add(ctx context.Context, jobID, ownerSub, ownerEmail, title string) error {
	if i == nil || i.client == nil {
		return nil
	}
	_, err := i.client.Collection(collectionJobs).Doc(jobID).Set(ctx, jobDoc{
		OwnerSub:   ownerSub,
		OwnerEmail: ownerEmail,
		Title:      title,
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		log.Error().Err(err).Str("job", jobID).Msg("firestore index add")
		return err
	}
	return nil
}

// ListRecent returns the most recently created jobs across all owners, newest first.
// Uses a single-field descending index on createdAt (auto-created by Firestore).
func (i *Index) ListRecent(ctx context.Context, limit int) ([]ports.JobMeta, error) {
	if i == nil || i.client == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	q := i.client.Collection(collectionJobs).
		OrderBy("createdAt", firestore.Desc).
		Limit(limit)
	iter := q.Documents(ctx)
	defer iter.Stop()
	var out []ports.JobMeta
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var d jobDoc
		if err := doc.DataTo(&d); err != nil {
			continue
		}
		out = append(out, ports.JobMeta{
			ID:        doc.Ref.ID,
			Title:     d.Title,
			CreatedAt: d.CreatedAt.UnixMilli(),
		})
	}
	return out, nil
}

// ListByOwner returns jobs for the given owner, newest first.
// Firestore requires a composite index: collection "jobs", fields ownerSub (Asc), createdAt (Desc).
func (i *Index) ListByOwner(ctx context.Context, ownerSub string, limit int) ([]ports.JobMeta, error) {
	if i == nil || i.client == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	q := i.client.Collection(collectionJobs).
		Where("ownerSub", "==", ownerSub).
		OrderBy("createdAt", firestore.Desc).
		Limit(limit)
	iter := q.Documents(ctx)
	defer iter.Stop()
	var out []ports.JobMeta
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var d jobDoc
		if err := doc.DataTo(&d); err != nil {
			continue
		}
		out = append(out, ports.JobMeta{
			ID:        doc.Ref.ID,
			Title:     d.Title,
			CreatedAt: d.CreatedAt.UnixMilli(),
		})
	}
	return out, nil
}
