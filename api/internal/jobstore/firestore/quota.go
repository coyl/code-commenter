package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"code-commenter/api/internal/ports"
)

const collectionDailyQuota = "daily_quota"

// Quota implements ports.DailyQuota using Firestore.
type Quota struct {
	client *firestore.Client
}

// NewQuota creates a Firestore daily quota store. projectID must be non-empty.
func NewQuota(ctx context.Context, projectID, databaseID string) (*Quota, error) {
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
	return &Quota{client: client}, nil
}

func todayKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

// GetTodayCount returns the number of generations today for the owner.
func (q *Quota) GetTodayCount(ctx context.Context, ownerSub string) (int, error) {
	if q == nil || q.client == nil || ownerSub == "" {
		return 0, nil
	}
	docRef := q.client.Collection(collectionDailyQuota).Doc(ownerSub + "_" + todayKey())
	doc, err := docRef.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var data struct {
		Count int `firestore:"count"`
	}
	if err := doc.DataTo(&data); err != nil {
		return 0, err
	}
	return data.Count, nil
}

// TryConsumeSlot atomically consumes one slot if under DailyGenerationLimit. Returns true if consumed, false if at limit.
func (q *Quota) TryConsumeSlot(ctx context.Context, ownerSub string) (bool, error) {
	if q == nil || q.client == nil || ownerSub == "" {
		return true, nil
	}
	docRef := q.client.Collection(collectionDailyQuota).Doc(ownerSub + "_" + todayKey())
	var consumed bool
	err := q.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		count := 0
		if err != nil && status.Code(err) != codes.NotFound {
			return err
		}
		if err == nil {
			var data struct {
				Count int `firestore:"count"`
			}
			if doc.DataTo(&data) == nil {
				count = data.Count
			}
		}
		if count >= ports.DailyGenerationLimit {
			consumed = false
			return nil
		}
		consumed = true
		return tx.Set(docRef, map[string]int{"count": count + 1})
	})
	return consumed, err
}

// ReleaseSlot decrements today's count by one (e.g. when generation failed after TryConsumeSlot).
func (q *Quota) ReleaseSlot(ctx context.Context, ownerSub string) error {
	if q == nil || q.client == nil || ownerSub == "" {
		return nil
	}
	docRef := q.client.Collection(collectionDailyQuota).Doc(ownerSub + "_" + todayKey())
	return q.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		count := 0
		if err != nil && status.Code(err) != codes.NotFound {
			return err
		}
		if err == nil {
			var data struct {
				Count int `firestore:"count"`
			}
			if doc.DataTo(&data) == nil {
				count = data.Count
			}
		}
		if count > 0 {
			count--
		}
		return tx.Set(docRef, map[string]int{"count": count})
	})
}

// Close releases the Firestore client.
func (q *Quota) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

var _ ports.DailyQuota = (*Quota)(nil)
