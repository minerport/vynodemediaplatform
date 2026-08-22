package metadata

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func normalize(s string) string {
	s = strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s))
	return strings.TrimSpace(strings.Join(strings.Fields(nonWord.ReplaceAllString(s, " ")), " "))
}

func Score(title string, year int, parent string, candidates []Candidate) Match {
	type scored struct {
		Candidate
		score   int
		signals []string
	}
	items := make([]scored, 0, len(candidates))
	for _, c := range candidates {
		s := scored{Candidate: c}
		candidate, hint := normalize(c.Title), normalize(title)
		if candidate == hint && hint != "" {
			s.score += 70
			s.signals = append(s.signals, "filename title exact normalized match")
		} else if hint != "" && (strings.Contains(candidate, hint) || strings.Contains(hint, candidate)) {
			s.score += 40
			s.signals = append(s.signals, "filename title similar")
		}
		if year > 0 && c.Year == year {
			s.score += 25
			s.signals = append(s.signals, "year exact match")
		} else if year > 0 && c.Year > 0 && (c.Year-year == 1 || year-c.Year == 1) {
			s.score += 8
			s.signals = append(s.signals, "year near match")
		}
		if p := normalize(parent); p != "" && (p == candidate || strings.HasPrefix(p, candidate+" ")) {
			s.score += 10
			s.signals = append(s.signals, "parent directory title match")
		}
		items = append(items, s)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].ProviderID < items[j].ProviderID
		}
		return items[i].score > items[j].score
	})
	r := Match{State: "UNMATCHED", Confidence: "LOW", Candidates: candidates}
	if len(items) == 0 {
		return r
	}
	r.Score, r.Signals = items[0].score, items[0].signals
	r.Candidate = &items[0].Candidate
	margin := items[0].score
	if len(items) > 1 {
		margin -= items[1].score
	}
	if r.Score >= 90 && margin >= 15 {
		r.State, r.Confidence = "MATCHED", "HIGH"
	} else if r.Score >= 60 {
		r.State, r.Confidence = "AMBIGUOUS", "MEDIUM"
	}
	return r
}
