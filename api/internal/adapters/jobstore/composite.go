package jobstore

import (
	"context"

	"code-commenter/api/internal/ports"
)

// CompositeRepository wraps a JobRepository and optionally writes to a JobIndex on each upload.
type CompositeRepository struct {
	Repo  ports.JobRepository
	Index ports.JobIndex
}

// UploadJob delegates to Repo and then adds metadata to Index when configured.
func (c *CompositeRepository) UploadJob(ctx context.Context, jobID, prompt, rawJSON, fullCode, fullCodePlain, css, title, narrationLang, ownerSub, ownerEmail, storyHTML string, images ports.JobImages, segments []ports.JobSegment, segmentAudio [][]byte) error {
	if err := c.Repo.UploadJob(ctx, jobID, prompt, rawJSON, fullCode, fullCodePlain, css, title, narrationLang, ownerSub, ownerEmail, storyHTML, images, segments, segmentAudio); err != nil {
		return err
	}
	if c.Index != nil {
		_ = c.Index.Add(ctx, jobID, ownerSub, ownerEmail, title)
	}
	return nil
}

// GetJob delegates to Repo.
func (c *CompositeRepository) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	return c.Repo.GetJob(ctx, jobID)
}

// IsEnabled returns true if the underlying Repo is enabled.
func (c *CompositeRepository) IsEnabled() bool {
	return c.Repo.IsEnabled()
}
