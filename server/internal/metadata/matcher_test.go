package metadata

import "testing"

func TestScoreRemakeUsesExactYear(t *testing.T) {
	r := Score("The Thing", 1982, "The Thing (1982)", []Candidate{{ProviderID: "1", Title: "The Thing", Year: 2011}, {ProviderID: "2", Title: "The Thing", Year: 1982}, {ProviderID: "3", Title: "Other", Year: 1982}})
	if r.State != "MATCHED" || r.Candidate == nil || r.Candidate.ProviderID != "2" {
		t.Fatalf("unexpected match: %+v", r)
	}
}
func TestScoreAmbiguousWithoutYear(t *testing.T) {
	r := Score("The Thing", 0, "", []Candidate{{ProviderID: "1", Title: "The Thing", Year: 2011}, {ProviderID: "2", Title: "The Thing", Year: 1982}})
	if r.State != "AMBIGUOUS" {
		t.Fatalf("expected ambiguous: %+v", r)
	}
}
func TestScorePoorCandidateUnmatched(t *testing.T) {
	r := Score("Unknown", 0, "", []Candidate{{ProviderID: "1", Title: "Different", Year: 2020}})
	if r.State != "UNMATCHED" {
		t.Fatalf("expected unmatched: %+v", r)
	}
}
