package truncate

import "regexp"

// TestFilter analyzes test output and filters PASS lines from the keepMap.
type TestFilter struct {
	summaryPats []*regexp.Regexp
	passPats    []*regexp.Regexp
	hasFailure  bool
}

// compilePattern compiles a regex pattern string.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// compilePatterns compiles a slice of pattern strings into regexps.
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := compilePattern(p)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}
	return compiled
}

// NewTestFilter creates a TestFilter by analyzing input lines and scores.
// scores from Scorer are used to detect FAIL lines (score > 0 = FAIL/ERROR match).
func NewTestFilter(lines []string, scores []int) *TestFilter {
	hasFailure := false
	for _, s := range scores {
		if s > 0 {
			hasFailure = true
			break
		}
	}
	return &TestFilter{
		summaryPats: compilePatterns(testSummaryPatterns()),
		passPats:    compilePatterns(testPassPatterns()),
		hasFailure:  hasFailure,
	}
}

// Apply modifies the keepMap: PASS lines become false, summary lines become true.
// Lines with score > 0 (FAIL/ERROR) are never removed (safety measure).
func (tf *TestFilter) Apply(keep []bool, lines []string, scores []int) []bool {
	result := make([]bool, len(keep))
	copy(result, keep)

	for i, line := range lines {
		if tf.matchesSummary(line) {
			result[i] = true
			continue
		}
		if scores[i] > 0 {
			continue // never remove FAIL/ERROR lines
		}
		if tf.matchesPass(line) {
			result[i] = false
		}
	}
	return result
}

func (tf *TestFilter) matchesSummary(line string) bool {
	for _, re := range tf.summaryPats {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func (tf *TestFilter) matchesPass(line string) bool {
	for _, re := range tf.passPats {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}
