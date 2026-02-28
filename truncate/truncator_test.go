package truncate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTruncate_EmptyInput(t *testing.T) {
	result := Truncate(nil, Options{Limit: 1000})
	assert.Equal(t, 0, result.TotalLines)
	assert.Equal(t, 0, result.KeptLines)
}

func TestTruncate_ShortCircuit(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20})

	assert.Equal(t, 3, result.TotalLines)
	assert.Equal(t, 3, result.KeptLines)
	assert.Equal(t, 0, result.OmittedLines)
	assert.Len(t, result.Blocks, 1)
	assert.Equal(t, "line 1\nline 2\nline 3", result.Blocks[0].Content)
}

func TestTruncate_BasicTruncation(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %d: everything is fine", i)
	}
	lines[50] = "ERROR: disk space critical"

	result := Truncate(lines, Options{
		Limit:   500,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "general",
	})

	assert.Equal(t, 100, result.TotalLines)
	assert.Greater(t, result.OmittedLines, 0)
	assert.Equal(t, 1, result.PatternsMatched)

	found := false
	for _, b := range result.Blocks {
		if strings.Contains(b.Content, "ERROR: disk space critical") {
			found = true
			break
		}
	}
	assert.True(t, found, "ERROR line should be preserved in output")
}

func TestTruncate_HeadAndTailPreserved(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	result := Truncate(lines, Options{
		Limit:   300,
		Head:    3,
		Tail:    3,
		Context: 0,
		Mode:    "general",
	})

	assert.True(t, strings.HasPrefix(result.Blocks[0].Content, "line 0"))
	lastKept := result.Blocks[len(result.Blocks)-1]
	if lastKept.Type != BlockOmitted {
		assert.True(t, strings.Contains(lastKept.Content, "line 49"))
	}
}

func TestTruncate_MultipleErrors(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("processing item %d", i)
	}
	lines[30] = "ERROR: first error"
	lines[100] = "FATAL: second error"
	lines[170] = "WARN: third warning"

	result := Truncate(lines, Options{
		Limit:   1000,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "general",
	})

	assert.Equal(t, 3, result.PatternsMatched)

	allContent := blocksToString(result.Blocks)
	assert.Contains(t, allContent, "ERROR: first error")
	assert.Contains(t, allContent, "FATAL: second error")
	assert.Contains(t, allContent, "WARN: third warning")
}

func TestTruncate_CustomPattern(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("data %d", i)
	}
	lines[25] = "CUSTOM_TAG: important info"

	result := Truncate(lines, Options{
		Limit:        300,
		Head:         2,
		Tail:         2,
		Context:      1,
		Mode:         "general",
		KeepPatterns: []string{`CUSTOM_TAG`},
	})

	allContent := blocksToString(result.Blocks)
	assert.Contains(t, allContent, "CUSTOM_TAG: important info")
}

func TestTruncate_ByteLimitStrictlyEnforced(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = strings.Repeat("x", 100)
	}
	lines[500] = "ERROR: " + strings.Repeat("y", 93)

	limit := 5000
	result := Truncate(lines, Options{
		Limit:   limit,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "general",
	})

	// Render as plain and verify it fits within limit
	out, _ := Render(result, FormatPlain)
	assert.LessOrEqual(t, len(out), limit, "rendered output must fit within byte limit")
	assert.Greater(t, result.OmittedLines, 0)
}

// Fix #2 test: invalid regex should NOT bypass limit
func TestTruncate_InvalidRegexRespectsLimit(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("ok line %d", i)
	}

	limit := 200
	result := Truncate(lines, Options{
		Limit:        limit,
		Head:         3,
		Tail:         3,
		Context:      1,
		Mode:         "general",
		KeepPatterns: []string{"[invalid"},
	})

	out, _ := Render(result, FormatPlain)
	assert.LessOrEqual(t, len(out), limit, "invalid regex must not bypass limit")
	assert.Greater(t, result.OmittedLines, 0)
}

// Fix #3 test: head+tail alone exceeds limit → anchors also get dropped
func TestTruncate_TinyLimitDropsAnchors(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = strings.Repeat("a", 50) // 50 bytes per line
	}

	limit := 100
	result := Truncate(lines, Options{
		Limit:   limit,
		Head:    10,
		Tail:    10,
		Context: 0,
		Mode:    "general",
	})

	out, _ := Render(result, FormatPlain)
	assert.LessOrEqual(t, len(out), limit, "tiny limit must be respected even with large head/tail")
}

// Fix off-by-one: renderedPlainBytes accounts for trailing newline from renderPlain.
// Uses FormatYAML to test the boundary directly (effective limit = limit for YAML).
func TestTruncate_ShortCircuitBoundary(t *testing.T) {
	// "aaa\nbbb" = 7 bytes content. renderPlain = "aaa\nbbb\n" = 8 bytes.
	lines := []string{"aaa", "bbb"}

	// limit=8, format=YAML (effective=8) exactly matches rendered output → short-circuit
	result8 := Truncate(lines, Options{Limit: 8, Head: 20, Tail: 20, Format: FormatYAML})
	assert.Equal(t, 2, result8.KeptLines)
	assert.Equal(t, 0, result8.OmittedLines, "limit=8 should short-circuit with no omission")

	// limit=7, format=YAML (effective=7) < rendered output (8) → must NOT short-circuit
	result7 := Truncate(lines, Options{Limit: 7, Head: 20, Tail: 20, Format: FormatYAML})
	assert.Less(t, result7.KeptLines, 2, "limit=7 must not short-circuit (rendered=8)")
}

// Fix format-aware limit: plain uses 90% of limit, YAML/JSON uses full limit.
func TestTruncate_FormatAwareLimit_Plain(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = strings.Repeat("x", 80)
	}
	lines[250] = "ERROR: " + strings.Repeat("y", 73)

	limit := 5000
	result := Truncate(lines, Options{
		Limit:   limit,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "general",
		Format:  FormatPlain,
	})

	out, _ := Render(result, FormatPlain)
	// plain text should be within limit*0.9
	plainLimit := int(float64(limit) * 0.9)
	assert.LessOrEqual(t, len(out), plainLimit,
		"plain output should fit within 90%% of limit (%d), got %d", plainLimit, len(out))
}

func TestTruncate_FormatAwareLimit_YAML(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = strings.Repeat("x", 80)
	}
	lines[250] = "ERROR: " + strings.Repeat("y", 73)

	limit := 5000
	result := Truncate(lines, Options{
		Limit:   limit,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "general",
		Format:  FormatYAML,
	})

	out, _ := Render(result, FormatYAML)
	assert.LessOrEqual(t, len(out), limit,
		"YAML output should fit within full limit (%d), got %d", limit, len(out))
}

func TestTruncate_FormatAwareLimit_JSON(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = strings.Repeat("x", 80)
	}
	lines[250] = "ERROR: " + strings.Repeat("y", 73)

	limit := 5000
	result := Truncate(lines, Options{
		Limit:   limit,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "general",
		Format:  FormatJSON,
	})

	out, _ := Render(result, FormatJSON)
	assert.LessOrEqual(t, len(out), limit,
		"JSON output should fit within full limit (%d), got %d", limit, len(out))
}

// Default format (zero value) should behave as plain
func TestTruncate_FormatDefault_IsPlain(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = strings.Repeat("x", 80)
	}

	limit := 5000
	result := Truncate(lines, Options{
		Limit: limit,
		Head:  5,
		Tail:  5,
		Mode:  "general",
		// Format not set → zero value = FormatPlain
	})

	out, _ := Render(result, FormatPlain)
	plainLimit := int(float64(limit) * 0.9)
	assert.LessOrEqual(t, len(out), plainLimit,
		"default format should use plain limit (90%%)")
}

// Fix #2: shortCircuit must set EffectiveLimit so Render can enforce limits
func TestTruncate_ShortCircuitSetsEffectiveLimit(t *testing.T) {
	lines := []string{"line 1", "line 2"}

	// YAML: effectiveLimit = limit (full)
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Format: FormatYAML})
	assert.Equal(t, 2, result.KeptLines, "should short-circuit")
	assert.Equal(t, 30000, result.EffectiveLimit,
		"short-circuit with YAML must set EffectiveLimit to full limit")

	// Plain: effectiveLimit = int(limit * 0.9)
	result2 := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Format: FormatPlain})
	assert.Equal(t, 2, result2.KeptLines, "should short-circuit")
	assert.Equal(t, 27000, result2.EffectiveLimit,
		"short-circuit with Plain must set EffectiveLimit to 90%% of limit")
}

// Fix #3: effectiveLimitForFormat must return at least 1 when limit > 0
func TestEffectiveLimitForFormat_MinimumOne(t *testing.T) {
	// int(1 * 0.9) = 0 without the fix → hardCut guard bypassed
	assert.GreaterOrEqual(t, effectiveLimitForFormat(1, FormatPlain), 1,
		"plain limit=1 must produce effectiveLimit >= 1")
	assert.GreaterOrEqual(t, effectiveLimitForFormat(1, FormatJSON), 1,
		"JSON limit=1 must produce effectiveLimit >= 1")
	assert.GreaterOrEqual(t, effectiveLimitForFormat(1, FormatYAML), 1,
		"YAML limit=1 must produce effectiveLimit >= 1")

	// limit=0 means "no limit" → should stay 0
	assert.Equal(t, 0, effectiveLimitForFormat(0, FormatPlain))
}

// Benchmark: enforceCharLimit performance with many lines
func BenchmarkTruncate_LargeInput(b *testing.B) {
	lines := make([]string, 10000)
	for i := range lines {
		lines[i] = strings.Repeat("x", 100)
	}
	// Sprinkle some errors
	lines[1000] = "ERROR: " + strings.Repeat("y", 93)
	lines[5000] = "FATAL: " + strings.Repeat("z", 93)
	lines[8000] = "WARN: " + strings.Repeat("w", 94)

	opts := Options{
		Limit:   5000,
		Head:    20,
		Tail:    20,
		Context: 3,
		Mode:    "general",
		Format:  FormatPlain,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Truncate(lines, opts)
	}
}

// Fix: extreme tiny limit where even omitted marker exceeds limit
func TestTruncate_ExtremeTinyLimit(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = strings.Repeat("x", 50)
	}

	// limit=10: no single line or marker can fit in 10 bytes
	limit := 10
	result := Truncate(lines, Options{
		Limit:   limit,
		Head:    5,
		Tail:    5,
		Context: 0,
		Mode:    "general",
		Format:  FormatYAML, // use YAML so effective limit = limit
	})

	out, _ := Render(result, FormatPlain)
	assert.LessOrEqual(t, len(out), limit,
		"extreme tiny limit: output must not exceed %d bytes, got %d", limit, len(out))
	assert.Contains(t, out, "...", "should have truncation indicator")
}

func TestTruncate_LimitSmallerThanMarker(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}

	// limit=5: even "...(truncated)" would be too long,
	// but we hard-cut at limit bytes
	limit := 5
	result := Truncate(lines, Options{
		Limit:  limit,
		Head:   2,
		Tail:   2,
		Mode:   "general",
		Format: FormatYAML,
	})

	out, _ := Render(result, FormatPlain)
	assert.LessOrEqual(t, len(out), limit,
		"limit=%d: output must not exceed limit, got %d bytes", limit, len(out))
}

// Benchmark: enforceCharLimit alone (excludes scoring overhead)
func BenchmarkEnforceCharLimit(b *testing.B) {
	total := 10000
	lines := make([]string, total)
	scores := make([]int, total)
	for i := range lines {
		lines[i] = strings.Repeat("x", 100)
	}
	lines[1000] = "ERROR: " + strings.Repeat("y", 93)
	scores[1000] = 1
	lines[5000] = "FATAL: " + strings.Repeat("z", 93)
	scores[5000] = 1

	opts := Options{
		Limit:  4500,
		Head:   20,
		Tail:   20,
		Format: FormatPlain,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keep := BuildKeepMap(total, scores, opts.Head, opts.Tail, 3)
		enforceCharLimit(lines, scores, keep, opts)
	}
}

// Issue 1: limit<=0 should mean unlimited (no truncation)
func TestTruncate_ZeroLimitMeansUnlimited(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	lines[50] = "ERROR: something bad"

	result := Truncate(lines, Options{Limit: 0, Head: 5, Tail: 5, Mode: "general"})
	assert.Equal(t, 100, result.TotalLines)
	assert.Equal(t, 100, result.KeptLines)
	assert.Equal(t, 0, result.OmittedLines)
	assert.Equal(t, 0, result.EffectiveLimit, "unlimited should have EffectiveLimit=0")
}

func TestTruncate_NegativeLimitMeansUnlimited(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	result := Truncate(lines, Options{Limit: -1, Head: 5, Tail: 5})
	assert.Equal(t, 50, result.TotalLines)
	assert.Equal(t, 50, result.KeptLines)
	assert.Equal(t, 0, result.OmittedLines)
}

// Issue 4: short-circuit should still count pattern matches
func TestTruncate_ShortCircuitPatternsMatched(t *testing.T) {
	lines := []string{"ok line", "ERROR: bad thing", "ok line 2", "WARN: be careful"}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "general"})
	assert.Equal(t, 4, result.KeptLines, "should short-circuit")
	assert.Equal(t, 2, result.PatternsMatched, "short-circuit should still count pattern matches")
}

// Issue 1+4 combined: unlimited with pattern matching
func TestTruncate_UnlimitedWithPatternsMatched(t *testing.T) {
	lines := []string{"ok", "ERROR: bad", "FATAL: worse"}
	result := Truncate(lines, Options{Limit: 0, Head: 5, Tail: 5, Mode: "general"})
	assert.Equal(t, 3, result.KeptLines)
	assert.Equal(t, 2, result.PatternsMatched, "unlimited mode should still count pattern matches")
}

// Issue 2 (5th review): negative context must not drop match lines
func TestTruncate_NegativeContextKeepsMatchLine(t *testing.T) {
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i+1)
	}
	lines[14] = "ERROR: boom"

	result := Truncate(lines, Options{
		Limit: 120, Head: 0, Tail: 0, Context: -1, Mode: "general",
	})

	allContent := blocksToString(result.Blocks)
	assert.Contains(t, allContent, "ERROR: boom", "match line must be kept even with negative context")
}

// Issue 1 (6th review): short-circuit + structured output boundary.
// When plain text fits but JSON/YAML metadata causes overflow,
// reduceBlocksForLimit must preserve match lines (not just head lines).
func TestTruncate_ShortCircuitStructuredPreservesMatchLine(t *testing.T) {
	lines := []string{"a", "ERROR: boom", "b"}
	// Full 3-line JSON ≈ 251 bytes (with type=match), 1-line match-only ≈ 245 bytes.
	// limit=250: plain (16 bytes) fits → short-circuit triggers,
	// but JSON metadata pushes output over 250 → reduceBlocksForLimit kicks in.
	// Bug: shortCircuit creates BlockHead → head strategy keeps "a", drops "ERROR: boom".
	// Fix: shortCircuit should classify as BlockMatch → match strategy centers on ERROR line.
	limit := 250
	result := Truncate(lines, Options{
		Limit: limit, Head: 0, Tail: 0, Context: 0, Mode: "general", Format: FormatJSON,
	})

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), limit)
	assert.Contains(t, out, "ERROR: boom",
		"match line must be preserved when plain fits but structured exceeds limit")
	assert.Equal(t, 1, result.PatternsMatched)

	// Also verify valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "must be valid JSON")
}

// Issue 1 (6th review): same bug with YAML format
func TestTruncate_ShortCircuitStructuredPreservesMatchLine_YAML(t *testing.T) {
	lines := []string{"a", "ERROR: boom", "b"}
	result := Truncate(lines, Options{
		Limit: 200, Head: 0, Tail: 0, Context: 0, Mode: "general", Format: FormatYAML,
	})

	out, err := Render(result, FormatYAML)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 200)
	assert.Contains(t, out, "ERROR: boom",
		"match line must be preserved in YAML when plain fits but structured exceeds limit")

	// Low fix (7th review): verify YAML is parseable (matching JSON side's Unmarshal check)
	var parsed map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "must be valid YAML")
}

// Low fix (7th review): shortCircuit must return BlockMatch when input contains match lines.
// Previous tests only verified via output string (Contains("ERROR: boom")).
// This test directly asserts result.Blocks[0].Type.
func TestTruncate_ShortCircuitBlockMatchType(t *testing.T) {
	lines := []string{"a", "ERROR: boom", "b"}
	result := Truncate(lines, Options{
		Limit: 30000, Head: 0, Tail: 0, Context: 0, Mode: "general",
	})

	assert.Equal(t, 3, result.KeptLines, "should short-circuit (all lines kept)")
	require.Len(t, result.Blocks, 1, "short-circuit should produce exactly 1 block")
	assert.Equal(t, BlockMatch, result.Blocks[0].Type,
		"short-circuit block with match line must be BlockMatch, got %q", result.Blocks[0].Type)
}

// Low fix (7th review): when no match line exists, shortCircuit should return BlockHead (or BlockTail).
func TestTruncate_ShortCircuitBlockHeadType(t *testing.T) {
	lines := []string{"a", "b", "c"}
	result := Truncate(lines, Options{
		Limit: 30000, Head: 20, Tail: 20, Context: 0, Mode: "general",
	})

	assert.Equal(t, 3, result.KeptLines, "should short-circuit")
	require.Len(t, result.Blocks, 1)
	assert.Equal(t, BlockHead, result.Blocks[0].Type,
		"short-circuit block without match should be BlockHead, got %q", result.Blocks[0].Type)
}

// Low fix (7th review): known limitation test for Medium-deferred issue.
// Condition: Head>0 && Tail>0, no-match, plain fits but structured exceeds limit.
// Current behavior: tail lines are dropped because shortCircuit produces a single
// BlockHead block and selectLinesToKeep keeps lines from the start (head strategy).
// This test pins the current behavior to prevent unintended regression.
func TestTruncate_KnownLimitation_ShortCircuitNoMatchStructuredTailDrop(t *testing.T) {
	// 5 lines of 20 bytes each, no pattern match.
	// Plain ≈ 105 bytes (fits within limit=300).
	// JSON ≈ 341 bytes (exceeds 300) → reduceBlocksForLimit triggers.
	lines := []string{
		strings.Repeat("a", 20),
		strings.Repeat("b", 20),
		strings.Repeat("c", 20),
		strings.Repeat("d", 20),
		strings.Repeat("e", 20),
	}
	limit := 300
	result := Truncate(lines, Options{
		Limit: limit, Head: 2, Tail: 2, Context: 0, Mode: "general", Format: FormatJSON,
	})

	// Pin the root cause: shortCircuit produces exactly 1 BlockHead block (no match → head classified).
	// This is what causes tail lines to be dropped during reduction.
	require.Len(t, result.Blocks, 1, "shortCircuit must produce exactly 1 block")
	assert.Equal(t, BlockHead, result.Blocks[0].Type,
		"no-match shortCircuit block must be BlockHead (root cause of tail drop)")

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), limit, "output must fit within limit")

	// Verify valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "must be valid JSON")

	// Known limitation: shortCircuit creates a single BlockHead (no match → head strategy).
	// reduceBlocksForLimit keeps head-side lines, dropping tail-side lines.
	// Head lines are preserved:
	assert.Contains(t, out, strings.Repeat("a", 20),
		"head lines should be preserved (current behavior)")
	// Tail lines are dropped despite Tail=2. This is the known limitation.
	assert.NotContains(t, out, strings.Repeat("e", 20),
		"known limitation: tail lines are dropped when single BlockHead is reduced")
}

func blocksToString(blocks []Block) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == BlockOmitted {
			parts = append(parts, fmt.Sprintf("... (%d lines omitted) ...", b.OmittedCount))
		} else {
			parts = append(parts, b.Content)
		}
	}
	return strings.Join(parts, "\n")
}

// Phase 3: TestFilter integration tests

func TestTruncate_TestMode_AllPass_GoSummaryOnly(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"=== RUN   TestSub",
		"--- PASS: TestSub (0.00s)",
		"=== RUN   TestMul",
		"--- PASS: TestMul (0.00s)",
		"ok  github.com/user/repo\t0.42s",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "test"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	assert.Contains(t, rendered, "ok  github.com/user/repo")
	assert.NotContains(t, rendered, "--- PASS:")
	assert.NotContains(t, rendered, "=== RUN")
}

func TestTruncate_TestMode_WithFailure_FailKept(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"=== RUN   TestSub",
		"--- FAIL: TestSub (0.00s)",
		"    sub_test.go:10: expected 3, got 4",
		"FAIL\tgithub.com/user/repo\t0.05s",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "test"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	assert.Contains(t, rendered, "--- FAIL: TestSub")
	assert.Contains(t, rendered, "FAIL\tgithub.com/user/repo")
	assert.NotContains(t, rendered, "--- PASS:")
}

func TestTruncate_TestMode_GeneralModeUnaffected(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"ok  github.com/user/repo\t0.42s",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "general"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	// general mode should keep PASS lines (no TestFilter applied)
	assert.Contains(t, rendered, "--- PASS:")
	assert.Contains(t, rendered, "=== RUN")
}

func TestTruncate_TestMode_BuildModeUnaffected(t *testing.T) {
	lines := []string{
		"=== RUN   TestAdd",
		"--- PASS: TestAdd (0.00s)",
		"ok  github.com/user/repo\t0.42s",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "build"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	assert.Contains(t, rendered, "--- PASS:")
}

func TestTruncate_TestMode_PytestAllPass(t *testing.T) {
	lines := []string{
		"test_add.py::test_add PASSED",
		"test_sub.py::test_sub PASSED",
		"test_mul.py::test_mul PASSED",
		"============================== 3 passed in 0.05s ==============================",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "test"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	assert.Contains(t, rendered, "3 passed")
	assert.NotContains(t, rendered, "test_add.py::test_add PASSED")
}

func TestTruncate_TestMode_JestAllPass(t *testing.T) {
	lines := []string{
		"  ✓ should add two numbers (3 ms)",
		"  ✓ should subtract (1 ms)",
		"  ✓ should multiply (1 ms)",
		"Tests:        3 passed, 3 total",
		"Test Suites:  1 passed, 1 total",
	}
	result := Truncate(lines, Options{Limit: 30000, Head: 20, Tail: 20, Mode: "test"})
	rendered, err := Render(result, FormatPlain)
	require.NoError(t, err)
	assert.Contains(t, rendered, "Tests:")
	assert.Contains(t, rendered, "Test Suites:")
	assert.NotContains(t, rendered, "✓ should add")
}
