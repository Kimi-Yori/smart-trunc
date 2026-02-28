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

func sampleResult() Result {
	return Result{
		TotalLines:      100,
		KeptLines:       20,
		OmittedLines:    80,
		PatternsMatched: 2,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 5, Content: "line 1\nline 2\nline 3\nline 4\nline 5"},
			{Type: BlockOmitted, StartLine: 6, EndLine: 45, OmittedCount: 40},
			{Type: BlockMatch, StartLine: 46, EndLine: 50, Content: "context\nERROR: bad\ncontext"},
			{Type: BlockOmitted, StartLine: 51, EndLine: 95, OmittedCount: 45},
			{Type: BlockTail, StartLine: 96, EndLine: 100, Content: "line 96\nline 97\nline 98\nline 99\nline 100"},
		},
	}
}

func TestRenderPlain_Basic(t *testing.T) {
	out, err := Render(sampleResult(), FormatPlain)
	require.NoError(t, err)

	assert.Contains(t, out, "line 1")
	assert.Contains(t, out, "ERROR: bad")
	assert.Contains(t, out, "line 100")
	assert.Contains(t, out, "... (40 lines omitted) ...")
	assert.Contains(t, out, "... (45 lines omitted) ...")
}

func TestRenderPlain_Empty(t *testing.T) {
	out, err := Render(Result{}, FormatPlain)
	require.NoError(t, err)
	assert.Equal(t, "", out)
}

func TestRenderJSON_Valid(t *testing.T) {
	out, err := Render(sampleResult(), FormatJSON)
	require.NoError(t, err)

	// Should be valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err)

	summary := parsed["summary"].(map[string]interface{})
	assert.Equal(t, float64(100), summary["total_lines"])
	assert.Equal(t, float64(20), summary["kept_lines"])
	assert.Equal(t, float64(80), summary["omitted_lines"])
}

func TestRenderYAML_Valid(t *testing.T) {
	out, err := Render(sampleResult(), FormatYAML)
	require.NoError(t, err)

	// Should contain YAML-style keys
	assert.Contains(t, out, "summary:")
	assert.Contains(t, out, "total_lines: 100")
	assert.Contains(t, out, "kept_lines: 20")
	assert.Contains(t, out, "blocks:")
}

// Fix #1: JSON output must remain valid JSON even when exceeding EffectiveLimit.
// Before fix: hardCut byte-slices JSON → broken syntax.
// After fix: block reduction + re-marshal → valid JSON.
func TestRender_JSONValidAfterReduction(t *testing.T) {
	// Build a result whose JSON rendering exceeds EffectiveLimit
	result := Result{
		TotalLines:      1000,
		KeptLines:       60,
		OmittedLines:    940,
		PatternsMatched: 3,
		EffectiveLimit:  500,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 20, Content: strings.Repeat("head line\n", 20)},
			{Type: BlockOmitted, StartLine: 21, EndLine: 480, OmittedCount: 460},
			{Type: BlockMatch, StartLine: 481, EndLine: 500, Content: strings.Repeat("ERROR: bad\n", 20)},
			{Type: BlockOmitted, StartLine: 501, EndLine: 980, OmittedCount: 480},
			{Type: BlockTail, StartLine: 981, EndLine: 1000, Content: strings.Repeat("tail line\n", 20)},
		},
	}

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 500, "JSON output must fit within EffectiveLimit")

	// Must be valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "JSON output must be parseable after reduction, got: %s", out)
}

// Fix #1: YAML output must remain valid YAML even when exceeding EffectiveLimit.
func TestRender_YAMLValidAfterReduction(t *testing.T) {
	result := Result{
		TotalLines:      1000,
		KeptLines:       60,
		OmittedLines:    940,
		PatternsMatched: 3,
		EffectiveLimit:  500,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 20, Content: strings.Repeat("head line\n", 20)},
			{Type: BlockOmitted, StartLine: 21, EndLine: 480, OmittedCount: 460},
			{Type: BlockMatch, StartLine: 481, EndLine: 500, Content: strings.Repeat("ERROR: bad\n", 20)},
			{Type: BlockOmitted, StartLine: 501, EndLine: 980, OmittedCount: 480},
			{Type: BlockTail, StartLine: 981, EndLine: 1000, Content: strings.Repeat("tail line\n", 20)},
		},
	}

	out, err := Render(result, FormatYAML)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 500, "YAML output must fit within EffectiveLimit")

	// Must be valid YAML
	var parsed map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "YAML output must be parseable after reduction, got: %s", out)
}

// Fix #4: Integration test — Truncate + Render produces parseable JSON
func TestTruncate_JSONOutputParseable(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %04d: %s", i, strings.Repeat("x", 80))
	}
	lines[250] = "ERROR: " + strings.Repeat("y", 93)

	limit := 800
	result := Truncate(lines, Options{
		Limit: limit, Head: 5, Tail: 5, Context: 2, Mode: "general", Format: FormatJSON,
	})

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), limit)

	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "Truncate+Render JSON must be parseable")
}

// Fix #4: Integration test — Truncate + Render produces parseable YAML
func TestTruncate_YAMLOutputParseable(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %04d: %s", i, strings.Repeat("x", 80))
	}
	lines[250] = "ERROR: " + strings.Repeat("y", 93)

	limit := 800
	result := Truncate(lines, Options{
		Limit: limit, Head: 5, Tail: 5, Context: 2, Mode: "general", Format: FormatYAML,
	})

	out, err := Render(result, FormatYAML)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), limit)

	var parsed map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "Truncate+Render YAML must be parseable")
}

// Issue 3: gradual block reduction instead of all-or-nothing
func TestRender_YAMLGradualBlockReduction(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	result := Truncate(lines, Options{
		Limit: 190, Head: 20, Tail: 20, Mode: "general", Format: FormatYAML,
	})

	out, err := Render(result, FormatYAML)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 190)

	var parsed yamlResult
	err = yaml.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err)

	assert.Greater(t, parsed.Summary.KeptLines, 0, "gradual reduction should keep some lines, not drop all to 0")
}

func TestRender_JSONGradualBlockReduction(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	result := Truncate(lines, Options{
		Limit: 239, Head: 20, Tail: 20, Mode: "general", Format: FormatJSON,
	})

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 239)

	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err, "must be valid JSON, got: %s", out)

	summary := parsed["summary"].(map[string]interface{})
	keptLines := int(summary["kept_lines"].(float64))
	assert.Greater(t, keptLines, 0, "gradual reduction should keep some lines")
}

// Issue 2: JSON/YAML must remain valid even at extreme limits (no hardCut for structured)
func TestRender_JSONValidAtTinyLimit(t *testing.T) {
	result := Result{
		TotalLines:      100,
		KeptLines:       20,
		OmittedLines:    80,
		PatternsMatched: 2,
		EffectiveLimit:  50,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 10, Content: strings.Repeat("x\n", 10)},
			{Type: BlockOmitted, StartLine: 11, EndLine: 100, OmittedCount: 90},
		},
	}

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)

	var parsed interface{}
	err = json.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "JSON must be valid even at tiny limit, got: %s", out)
}

func TestRender_YAMLValidAtTinyLimit(t *testing.T) {
	result := Result{
		TotalLines:      100,
		KeptLines:       20,
		OmittedLines:    80,
		PatternsMatched: 2,
		EffectiveLimit:  50,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 10, Content: strings.Repeat("x\n", 10)},
			{Type: BlockOmitted, StartLine: 11, EndLine: 100, OmittedCount: 90},
		},
	}

	out, err := Render(result, FormatYAML)
	require.NoError(t, err)

	var parsed interface{}
	err = yaml.Unmarshal([]byte(out), &parsed)
	assert.NoError(t, err, "YAML must be valid even at tiny limit, got: %s", out)
}

// Issue 1 (5th review): BlockMatch reduction must preserve the actual match line
func TestRender_JSONReducePreservesMatchLine(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("context line %d: abcdefghijklmnopqrstuvwxyz", i+1)
	}
	lines[19] = "ERROR: critical failure happened at subsystem"

	// limit=600: large enough for JSON with 1 match line + 2 omitted blocks,
	// but too small for the full match block (11 lines of context + error)
	result := Truncate(lines, Options{
		Limit: 600, Head: 0, Tail: 0, Context: 5, Mode: "general", Format: FormatJSON,
	})

	out, err := Render(result, FormatJSON)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(out), 600)
	assert.Contains(t, out, "ERROR: critical failure", "match line must be preserved after structured reduction")
}

// Issue 3 (5th review): structured format with too-small limit must return error, not empty success
func TestRender_JSONTooSmallReturnsError(t *testing.T) {
	result := Result{
		TotalLines:     10,
		KeptLines:      5,
		OmittedLines:   5,
		EffectiveLimit: 2,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 5, Content: "a\nb\nc\nd\ne"},
		},
	}

	_, err := Render(result, FormatJSON)
	assert.Error(t, err, "should return error when limit is too small for structured output")
}

func TestRender_YAMLTooSmallReturnsError(t *testing.T) {
	result := Result{
		TotalLines:     10,
		KeptLines:      5,
		OmittedLines:   5,
		EffectiveLimit: 2,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 5, Content: "a\nb\nc\nd\ne"},
		},
	}

	_, err := Render(result, FormatYAML)
	assert.Error(t, err, "should return error when limit is too small for structured output")
}

func TestRenderPlain_NoOmission(t *testing.T) {
	r := Result{
		TotalLines: 3,
		KeptLines:  3,
		Blocks: []Block{
			{Type: BlockHead, StartLine: 1, EndLine: 3, Content: "a\nb\nc"},
		},
	}
	out, err := Render(r, FormatPlain)
	require.NoError(t, err)
	assert.Equal(t, "a\nb\nc\n", out)
	assert.False(t, strings.Contains(out, "omitted"))
}
