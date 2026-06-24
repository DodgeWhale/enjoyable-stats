package analyser

import "testing"

func TestHalfScores_derivesSecondHalfFromHalftimeSnapshot(t *testing.T) {
	s := &State{
		halftimeSeen: true,
		halftimeCT:   4,
		halftimeT:    8,
	}

	fhCT, fhT, shCT, shT := s.halfScores(11, 13)
	if fhCT != 4 || fhT != 8 {
		t.Fatalf("first half = CT %d T %d, want 4-8", fhCT, fhT)
	}
	if shCT != 7 || shT != 5 {
		t.Fatalf("second half = CT %d T %d, want 7-5", shCT, shT)
	}
}

func TestHalfScores_emptyWhenHalftimeUnknown(t *testing.T) {
	s := &State{}
	fhCT, fhT, shCT, shT := s.halfScores(11, 13)
	if fhCT != 0 || fhT != 0 || shCT != 0 || shT != 0 {
		t.Fatalf("expected zero half scores, got %d-%d, %d-%d", fhCT, fhT, shCT, shT)
	}
}

func TestFinishFirstHalf_recordsSnapshot(t *testing.T) {
	s := &State{lastRoundOfFirstHalf: true}
	s.finishFirstHalf(3, 9)
	if !s.halftimeSeen || s.halftimeCT != 3 || s.halftimeT != 9 {
		t.Fatalf("halftime snapshot = CT %d T %d seen=%v", s.halftimeCT, s.halftimeT, s.halftimeSeen)
	}
	if s.lastRoundOfFirstHalf {
		t.Fatal("expected last-round flag to be cleared")
	}
}
