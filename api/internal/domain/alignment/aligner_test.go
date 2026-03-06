package alignment

import (
	"errors"
	"testing"
)

func TestAlignPairsAudioByIndex(t *testing.T) {
	svc := Service{}
	segments := []Segment{
		{Index: 0, Code: "a", Narration: "n1"},
		{Index: 1, Code: "b", Narration: "n2"},
	}
	audio := map[int]SegmentAudio{
		1: {Index: 1, Chunks: []string{"chunk-b"}},
		0: {Index: 0, Chunks: []string{"chunk-a"}},
	}

	got, err := svc.Align(segments, audio)
	if err != nil {
		t.Fatalf("Align() err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Segment.Index != 0 || len(got[0].Audio) != 1 || got[0].Audio[0] != "chunk-a" {
		t.Fatalf("unexpected aligned[0]: %+v", got[0])
	}
	if got[1].Segment.Index != 1 || len(got[1].Audio) != 1 || got[1].Audio[0] != "chunk-b" {
		t.Fatalf("unexpected aligned[1]: %+v", got[1])
	}
}

func TestAlignRejectsNonDeterministicOrdering(t *testing.T) {
	svc := Service{}
	segments := []Segment{
		{Index: 1},
	}
	_, err := svc.Align(segments, map[int]SegmentAudio{
		1: {Index: 1, Err: errors.New("boom")},
	})
	if err == nil {
		t.Fatal("expected error for non-deterministic ordering")
	}
}
