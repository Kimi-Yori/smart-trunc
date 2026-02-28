package truncate

import (
	"regexp"
)

// Scorer assigns importance scores to lines.
type Scorer struct {
	patterns []*regexp.Regexp
}

// NewScorer creates a Scorer from a mode name and optional extra patterns.
// Invalid extra patterns are individually skipped; valid ones are retained.
// Mode patterns are assumed valid (they are hardcoded constants).
func NewScorer(mode string, extraPatterns []string) (*Scorer, error) {
	modePatterns := ModePatterns(mode)

	compiled := make([]*regexp.Regexp, 0, len(modePatterns)+len(extraPatterns))
	for _, p := range modePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err // mode patterns must be valid
		}
		compiled = append(compiled, re)
	}
	for _, p := range extraPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue // skip invalid extra patterns
		}
		compiled = append(compiled, re)
	}
	return &Scorer{patterns: compiled}, nil
}

// Score returns the importance score for a single line.
// 0 = no match, higher = more important.
func (s *Scorer) Score(line string) int {
	score := 0
	for _, re := range s.patterns {
		if re.MatchString(line) {
			score++
		}
	}
	return score
}

// ScoreLines returns scores for all lines.
func (s *Scorer) ScoreLines(lines []string) []int {
	scores := make([]int, len(lines))
	for i, line := range lines {
		scores[i] = s.Score(line)
	}
	return scores
}
