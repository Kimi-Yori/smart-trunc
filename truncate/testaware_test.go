package truncate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Phase 1: パターン定義テスト

func TestTestSummaryPatterns_GoOk(t *testing.T) {
	pats := testSummaryPatterns()
	matched := matchesAny(pats, "ok  github.com/user/repo\t0.42s")
	assert.True(t, matched, "Go ok line should match summary pattern")
}

func TestTestSummaryPatterns_GoFail(t *testing.T) {
	pats := testSummaryPatterns()
	matched := matchesAny(pats, "FAIL\tgithub.com/user/repo\t0.05s")
	assert.True(t, matched, "Go FAIL line should match summary pattern")
}

func TestTestSummaryPatterns_Pytest(t *testing.T) {
	pats := testSummaryPatterns()
	matched := matchesAny(pats, "1 failed, 24 passed, 2 warnings in 3.45s")
	assert.True(t, matched, "pytest summary should match")
}

func TestTestSummaryPatterns_Jest(t *testing.T) {
	pats := testSummaryPatterns()
	matched := matchesAny(pats, "Tests:        2 failed, 18 passed, 20 total")
	assert.True(t, matched, "jest Tests summary should match")
}

func TestTestSummaryPatterns_JestSuites(t *testing.T) {
	pats := testSummaryPatterns()
	matched := matchesAny(pats, "Test Suites:  1 failed, 5 passed, 6 total")
	assert.True(t, matched, "jest Test Suites summary should match")
}

func TestTestPassPatterns_GoPass(t *testing.T) {
	pats := testPassPatterns()
	matched := matchesAny(pats, "--- PASS: TestAdd (0.00s)")
	assert.True(t, matched, "Go PASS line should match")
}

func TestTestPassPatterns_GoRun(t *testing.T) {
	pats := testPassPatterns()
	matched := matchesAny(pats, "=== RUN   TestAdd")
	assert.True(t, matched, "Go RUN line should match")
}

func TestTestPassPatterns_Pytest(t *testing.T) {
	pats := testPassPatterns()
	matched := matchesAny(pats, "test_add.py::test_add PASSED")
	assert.True(t, matched, "pytest PASSED line should match")
}

func TestTestPassPatterns_Jest(t *testing.T) {
	pats := testPassPatterns()
	matched := matchesAny(pats, "  ✓ should add two numbers (3 ms)")
	assert.True(t, matched, "jest checkmark line should match")
}

func TestNewTestFilter_DetectsFailure(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"=== RUN   TestSub",
		"--- FAIL: TestSub (0.00s)",
		"ok  github.com/user/repo",
	}
	scores := []int{0, 0, 0, 2, 0} // FAIL行はscores>0
	tf := NewTestFilter(lines, scores)
	assert.True(t, tf.hasFailure, "should detect failure when scores>0 exist")
}

func TestNewTestFilter_AllPass(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"ok  github.com/user/repo",
	}
	scores := []int{0, 0, 0}
	tf := NewTestFilter(lines, scores)
	assert.False(t, tf.hasFailure, "should not detect failure when all scores are 0")
}

// Phase 2: Apply関数テスト

func TestApply_RemovesGoPassLines(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"=== RUN   TestSub",
		"--- PASS: TestSub (0.00s)",
		"ok  github.com/user/repo\t0.42s",
	}
	scores := []int{0, 0, 0, 0, 0}
	keep := []bool{true, true, true, true, true}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.False(t, result[1], "--- PASS: line should be removed")
	assert.False(t, result[3], "--- PASS: line should be removed")
}

func TestApply_RemovesGoRunLines(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"ok  github.com/user/repo",
	}
	scores := []int{0, 0, 0}
	keep := []bool{true, true, true}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.False(t, result[0], "=== RUN line should be removed")
}

func TestApply_PreservesGoSummary(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"ok  github.com/user/repo\t0.42s",
	}
	scores := []int{0, 0, 0}
	keep := []bool{false, false, false} // even if BuildKeepMap set false
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.True(t, result[2], "summary line should be kept even if originally false")
}

func TestApply_PreservesFailLines(t *testing.T) {
	lines := []string{
		"=== RUN   TestSub",
		"--- FAIL: TestSub (0.00s)",
		"    sub_test.go:10: expected 3, got 4",
		"FAIL\tgithub.com/user/repo\t0.05s",
	}
	scores := []int{0, 2, 0, 2} // FAIL lines have score > 0
	keep := []bool{true, true, true, true}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.True(t, result[1], "FAIL line (score>0) should not be removed")
	assert.True(t, result[3], "FAIL summary should be kept")
}

func TestApply_HeadPassRemoved(t *testing.T) {
	// head=2 means lines 0,1 are in head range; PASS line in head should still be removed
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"=== RUN   TestSub",
		"--- PASS: TestSub (0.00s)",
		"ok  github.com/user/repo",
	}
	scores := []int{0, 0, 0, 0, 0}
	// Simulate BuildKeepMap with head=2: lines 0,1 are true
	keep := []bool{true, true, false, false, false}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.False(t, result[0], "RUN line in head range should be removed")
	assert.False(t, result[1], "PASS line in head range should be removed")
}

func TestApply_TailPassRemoved(t *testing.T) {
	lines := []string{
		"some output",
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"ok  github.com/user/repo",
	}
	scores := []int{0, 0, 0, 0}
	// Simulate tail=2: lines 2,3 in tail range
	keep := []bool{false, false, true, true}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.False(t, result[2], "PASS line in tail range should be removed")
	assert.True(t, result[3], "summary line in tail should be kept")
}

func TestApply_NonTestLinesUnchanged(t *testing.T) {
	lines := []string{
		"building project...",
		"compiling main.go",
		"linking...",
		"ok  github.com/user/repo",
	}
	scores := []int{0, 0, 0, 0}
	keep := []bool{true, false, true, true}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.True(t, result[0], "non-test line should be unchanged")
	assert.False(t, result[1], "non-test line should be unchanged")
	assert.True(t, result[2], "non-test line should be unchanged")
}

func TestApply_EmptyInput(t *testing.T) {
	lines := []string{}
	scores := []int{}
	keep := []bool{}
	tf := NewTestFilter(lines, scores)
	result := tf.Apply(keep, lines, scores)
	assert.Empty(t, result, "empty input should return empty result")
}

// Phase 5: Edge case tests

func TestTruncate_TestMode_NonTestOutput(t *testing.T) {
	// Non-test output passed to test mode should not break
	lines := []string{
		"Starting server on port 8080...",
		"Connected to database",
		"Listening for connections",
		"Server ready",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "test"})
	assert.Equal(t, 4, result.KeptLines, "non-test output should be fully kept")
}

func TestTruncate_TestMode_NoSummaryLine(t *testing.T) {
	// Incomplete test output with no summary
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"=== RUN   TestSub",
		"--- PASS: TestSub (0.00s)",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "test"})
	// All are PASS/RUN lines, should all be removed
	assert.Equal(t, 0, result.KeptLines, "all PASS/RUN lines should be removed")
}

func TestTestPassPatterns_NoFalsePositive_ErrorLine(t *testing.T) {
	// "ERROR" should not match PASS patterns
	pats := testPassPatterns()
	assert.False(t, matchesAny(pats, "ERROR: connection failed"), "ERROR line should not match PASS pattern")
}

func TestTestPassPatterns_NoFalsePositive_NormalLog(t *testing.T) {
	pats := testPassPatterns()
	assert.False(t, matchesAny(pats, "2026-02-16 processing batch 42"), "normal log should not match PASS pattern")
}

func TestTruncate_TestMode_WithTinyLimit(t *testing.T) {
	lines := make([]string, 50)
	for i := 0; i < 48; i++ {
		lines[i] = fmt.Sprintf("--- PASS: Test%d (0.00s)", i)
	}
	lines[48] = "PASS"
	lines[49] = "ok  github.com/user/repo\t0.42s"
	result := Truncate(lines, Options{Limit: 100, Head: 3, Tail: 3, Context: 1, Mode: "test"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	// Summary should still be present even with tiny limit
	assert.Contains(t, rendered, "ok  github.com/user/repo")
}

// matchesAny is a test helper: returns true if any pattern matches the line.
func matchesAny(patterns []string, line string) bool {
	for _, p := range patterns {
		re, err := compilePattern(p)
		if err != nil {
			continue
		}
		if re.MatchString(line) {
			return true
		}
	}
	return false
}
