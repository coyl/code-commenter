package alignment

import "fmt"

// Segment is a rendered/plain segment ready for emission.
type Segment struct {
	Index     int
	Code      string
	CodePlain string
	Narration string
}

// SegmentAudio represents generated audio chunks for a segment.
type SegmentAudio struct {
	Index  int
	Chunks []string
	Err    error
}

// AlignedSegment is a deterministic segment + audio pairing.
type AlignedSegment struct {
	Segment Segment
	Audio   []string
	Err     error
}

// Service aligns code segments and audio outputs by index.
type Service struct{}

// Align validates deterministic ordering and pairs audio by index.
func (Service) Align(segments []Segment, audioByIndex map[int]SegmentAudio) ([]AlignedSegment, error) {
	aligned := make([]AlignedSegment, 0, len(segments))
	for i, s := range segments {
		if s.Index != i {
			return nil, fmt.Errorf("non-deterministic segment ordering at %d (index=%d)", i, s.Index)
		}
		res := AlignedSegment{Segment: s}
		if audio, ok := audioByIndex[s.Index]; ok {
			res.Audio = audio.Chunks
			res.Err = audio.Err
		}
		aligned = append(aligned, res)
	}
	return aligned, nil
}
