package datastore

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"

	"code-commenter/api/internal/ports"
)

const kindDailyQuota = "DailyQuota"

type quotaEntity struct {
	Count int `datastore:"count"`
}

// Quota implements ports.DailyQuota using the same Datastore client pattern as Index.
type Quota struct {
	client *datastore.Client
}

// NewQuota creates a Datastore daily quota store. projectID must be non-empty.
func NewQuota(ctx context.Context, projectID, databaseID string) (*Quota, error) {
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
	key := datastore.NameKey(kindDailyQuota, ownerSub+"_"+todayKey(), nil)
	var ent quotaEntity
	err := q.client.Get(ctx, key, &ent)
	if err == datastore.ErrNoSuchEntity {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return ent.Count, nil
}

// IncrementToday increments today's generation count for the owner.
func (q *Quota) IncrementToday(ctx context.Context, ownerSub string) error {
	if q == nil || q.client == nil || ownerSub == "" {
		return nil
	}
	key := datastore.NameKey(kindDailyQuota, ownerSub+"_"+todayKey(), nil)
	_, err := q.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var ent quotaEntity
		if err := tx.Get(key, &ent); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		ent.Count++
		_, err := tx.Put(key, &ent)
		return err
	})
	return err
}

// TryConsumeSlot atomically consumes one slot if under DailyGenerationLimit. Returns true if consumed, false if at limit.
func (q *Quota) TryConsumeSlot(ctx context.Context, ownerSub string) (bool, error) {
	if q == nil || q.client == nil || ownerSub == "" {
		return true, nil
	}
	key := datastore.NameKey(kindDailyQuota, ownerSub+"_"+todayKey(), nil)
	var consumed bool
	_, err := q.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var ent quotaEntity
		if err := tx.Get(key, &ent); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if ent.Count >= ports.DailyGenerationLimit {
			consumed = false
			return nil
		}
		ent.Count++
		consumed = true
		_, err := tx.Put(key, &ent)
		return err
	})
	return consumed, err
}

// ReleaseSlot decrements today's count by one (e.g. when generation failed after TryConsumeSlot).
func (q *Quota) ReleaseSlot(ctx context.Context, ownerSub string) error {
	if q == nil || q.client == nil || ownerSub == "" {
		return nil
	}
	key := datastore.NameKey(kindDailyQuota, ownerSub+"_"+todayKey(), nil)
	_, err := q.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var ent quotaEntity
		if err := tx.Get(key, &ent); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if ent.Count > 0 {
			ent.Count--
		}
		_, err := tx.Put(key, &ent)
		return err
	})
	return err
}

// Close releases the client. No-op if Quota was created from shared client.
func (q *Quota) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

// Ensure Quota implements the interface.
var _ ports.DailyQuota = (*Quota)(nil)
