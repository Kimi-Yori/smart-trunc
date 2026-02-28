package truncate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScorer_ValidPatterns(t *testing.T) {
	s, err := NewScorer("general", nil)
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestNewScorer_InvalidPatternSkipped(t *testing.T) {
	// Invalid extra pattern is skipped, not an error
	s, err := NewScorer("general", []string{"[invalid"})
	require.NoError(t, err)
	// Mode patterns still work
	assert.Greater(t, s.Score("ERROR: something"), 0)
}

func TestScorer_ErrorLineScoresPositive(t *testing.T) {
	s, _ := NewScorer("general", nil)
	assert.Greater(t, s.Score("ERROR: something failed"), 0)
	assert.Greater(t, s.Score("fatal error occurred"), 0)
	assert.Greater(t, s.Score("PANIC: runtime error"), 0)
}

func TestScorer_NormalLineScoresZero(t *testing.T) {
	s, _ := NewScorer("general", nil)
	assert.Equal(t, 0, s.Score("INFO: server started on port 8080"))
	assert.Equal(t, 0, s.Score("processing item 42..."))
	assert.Equal(t, 0, s.Score(""))
}

func TestScorer_TestModeMatchesFAIL(t *testing.T) {
	s, _ := NewScorer("test", nil)
	assert.Greater(t, s.Score("--- FAIL: TestSomething (0.01s)"), 0)
	assert.Greater(t, s.Score("FAIL	mypackage	0.005s"), 0)
}

func TestScorer_BuildModeMatchesNpmErr(t *testing.T) {
	s, _ := NewScorer("build", nil)
	assert.Greater(t, s.Score("npm ERR! code ERESOLVE"), 0)
	assert.Greater(t, s.Score("error: cannot find module 'foo'"), 0)
}

func TestScorer_CustomPattern(t *testing.T) {
	s, _ := NewScorer("general", []string{`CUSTOM_MARKER`})
	assert.Greater(t, s.Score("CUSTOM_MARKER: attention here"), 0)
	assert.Equal(t, 0, s.Score("nothing special"))
}

func TestScorer_MultipleMatchesStackScore(t *testing.T) {
	// A line matching multiple patterns gets a higher score
	s, _ := NewScorer("test", nil)
	// "FAIL" matches (?i)FAIL, and also contains "ERROR"
	score := s.Score("FAIL: ERROR in test")
	assert.Greater(t, score, 1)
}

// Fix: invalid regex should be skipped individually, valid ones retained
func TestNewScorer_PartialInvalidPattern(t *testing.T) {
	// One valid, one invalid → valid pattern should still work
	s, err := NewScorer("general", []string{"VALID_MARKER", "[invalid"})
	require.NoError(t, err, "partial invalid should not return error")
	assert.Greater(t, s.Score("VALID_MARKER: found it"), 0, "valid extra pattern should match")
}

func TestNewScorer_AllInvalidPatterns(t *testing.T) {
	// All extra patterns invalid → should still work with mode patterns only
	s, err := NewScorer("general", []string{"[bad1", "[bad2"})
	require.NoError(t, err, "all-invalid extras should not return error")
	assert.Greater(t, s.Score("ERROR: something"), 0, "mode patterns should still work")
	assert.Equal(t, 0, s.Score("normal line"), "non-matching line should score 0")
}

func TestScoreLines(t *testing.T) {
	s, _ := NewScorer("general", nil)
	lines := []string{
		"starting process",
		"ERROR: disk full",
		"retrying...",
		"FATAL: giving up",
		"done",
	}
	scores := s.ScoreLines(lines)
	assert.Equal(t, 5, len(scores))
	assert.Equal(t, 0, scores[0])
	assert.Greater(t, scores[1], 0) // ERROR
	assert.Equal(t, 0, scores[2])
	assert.Greater(t, scores[3], 0) // FATAL
	assert.Equal(t, 0, scores[4])
}
