package jobstore

import (
	"context"

	core "code-commenter/api/internal/jobstore"
	"code-commenter/api/internal/ports"
)

// Adapter maps S3 jobstore to job repository port.
type Adapter struct {
	Store *core.Client
}

func (a *Adapter) UploadJob(ctx context.Context, jobID, prompt, rawJSON, fullCode, fullCodePlain, css, title, narrationLang string, segments []ports.JobSegment, segmentAudio [][]byte) error {
	if a.Store == nil {
		return nil
	}
	stored := make([]core.SegmentStored, 0, len(segments))
	for _, seg := range segments {
		stored = append(stored, core.SegmentStored{
			Code:      seg.Code,
			CodePlain: seg.CodePlain,
			Narration: seg.Narration,
		})
	}
	return a.Store.UploadJob(ctx, jobID, prompt, rawJSON, fullCode, fullCodePlain, css, title, narrationLang, stored, segmentAudio)
}

func (a *Adapter) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	if a.Store == nil {
		return nil, context.Canceled
	}
	return a.Store.GetJob(ctx, jobID)
}

func (a *Adapter) IsEnabled() bool {
	return a.Store != nil
}

// NoopAdapter allows running without S3 configured.
type NoopAdapter struct{}

func (NoopAdapter) UploadJob(context.Context, string, string, string, string, string, string, string, string, []ports.JobSegment, [][]byte) error {
	return nil
}

func (NoopAdapter) GetJob(context.Context, string) (interface{}, error) {
	return nil, context.Canceled
}

func (NoopAdapter) IsEnabled() bool {
	return false
}
