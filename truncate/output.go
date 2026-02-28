package truncate

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const hardCutSuffix = "...(truncated)\n"

// Render formats a Result into the specified output format.
// For plain text, hard-cut is used when output exceeds EffectiveLimit.
// For JSON/YAML, blocks are progressively reduced to maintain valid syntax.
func Render(result Result, format OutputFormat) (string, error) {
	var out string
	var err error

	switch format {
	case FormatJSON:
		out, err = renderJSON(result)
	case FormatYAML:
		out, err = renderYAML(result)
	default:
		out = renderPlain(result)
	}
	if err != nil {
		return "", err
	}

	if result.EffectiveLimit > 0 && len(out) > result.EffectiveLimit {
		switch format {
		case FormatJSON, FormatYAML:
			// Structured formats: reduce blocks to maintain valid syntax
			return reduceBlocksForLimit(result, format, result.EffectiveLimit)
		default:
			// Plain text: hard-cut is safe (no structural grammar)
			out = hardCut(out, result.EffectiveLimit)
		}
	}

	return out, nil
}

// hardCut truncates output to fit within limit bytes, appending a truncation indicator.
func hardCut(out string, limit int) string {
	if limit >= len(hardCutSuffix) {
		// Room for suffix: cut content + append suffix
		cutAt := limit - len(hardCutSuffix)
		return out[:cutAt] + hardCutSuffix
	}
	// Extremely tiny limit: just cut at limit bytes
	if limit > 0 {
		return out[:limit]
	}
	return ""
}

// reduceBlocksForLimit progressively trims content blocks until the
// structured output (JSON/YAML) fits within the byte limit.
// Uses binary search within each block to find the maximum lines that fit,
// without adding intermediate omitted blocks (which inflate metadata size).
// Removal priority: tail → head → match (preserving the most important content).
func reduceBlocksForLimit(result Result, format OutputFormat, limit int) (string, error) {
	blocks := make([]Block, len(result.Blocks))
	copy(blocks, result.Blocks)

	for {
		idx := findLowestPriorityContentBlock(blocks)
		if idx < 0 {
			break // no more content blocks
		}

		b := blocks[idx]
		contentLines := strings.Split(b.Content, "\n")

		// Binary search: find max lines to keep (1..len) that fits in limit
		bestFit := 0
		lo, hi := 1, len(contentLines)
		for lo <= hi {
			mid := (lo + hi) / 2
			testBlocks := makeTestBlocks(blocks, idx, b, contentLines, mid, result.LineScores)
			reduced := rebuildResult(result, testBlocks)
			out, err := renderStructured(reduced, format)
			if err != nil {
				return "", err
			}
			if len(out) <= limit {
				bestFit = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}

		if bestFit > 0 {
			// Apply the same line-selection strategy as makeTestBlocks
			start, end := selectLinesToKeep(b.Type, contentLines, bestFit, b.StartLine, result.LineScores)
			blocks[idx] = Block{
				Type:      b.Type,
				StartLine: b.StartLine + start,
				EndLine:   b.StartLine + end - 1,
				Content:   strings.Join(contentLines[start:end], "\n"),
				Pattern:   b.Pattern,
			}
			reduced := rebuildResult(result, blocks)
			return renderStructured(reduced, format)
		}

		// No content lines fit — convert entire block to omitted
		blocks[idx] = Block{
			Type:         BlockOmitted,
			StartLine:    b.StartLine,
			EndLine:      b.EndLine,
			OmittedCount: b.EndLine - b.StartLine + 1,
		}
		blocks = mergeAdjacentOmitted(blocks)

		reduced := rebuildResult(result, blocks)
		out, err := renderStructured(reduced, format)
		if err != nil {
			return "", err
		}
		if len(out) <= limit {
			return out, nil
		}
		// Continue removing other blocks
	}

	// All content blocks removed — try summary-only (empty blocks)
	summaryOnly := rebuildResult(result, []Block{})
	out, err := renderStructured(summaryOnly, format)
	if err != nil {
		return "", err
	}
	if len(out) <= limit {
		return out, nil
	}

	// Even summary-only doesn't fit — return minimal valid structure
	return minimalValidStructure(format, limit)
}

// makeTestBlocks creates a copy of blocks with the block at idx replaced
// by a trimmed version keeping only keepN lines of content.
// Uses selectLinesToKeep to choose which lines based on block type and scores.
func makeTestBlocks(blocks []Block, idx int, original Block, contentLines []string, keepN int, scores []int) []Block {
	testBlocks := make([]Block, len(blocks))
	copy(testBlocks, blocks)
	start, end := selectLinesToKeep(original.Type, contentLines, keepN, original.StartLine, scores)
	testBlocks[idx] = Block{
		Type:      original.Type,
		StartLine: original.StartLine + start,
		EndLine:   original.StartLine + end - 1,
		Content:   strings.Join(contentLines[start:end], "\n"),
		Pattern:   original.Pattern,
	}
	return testBlocks
}

// selectLinesToKeep returns the [start, end) slice indices for which lines
// to keep from a content block.
// - BlockMatch: centers around the highest-score line (preserves the actual match).
// - BlockTail: keeps from the end.
// - BlockHead/others: keeps from the start.
func selectLinesToKeep(blockType string, contentLines []string, keepN int, startLine int, scores []int) (int, int) {
	total := len(contentLines)
	if keepN >= total {
		return 0, total
	}

	switch blockType {
	case BlockMatch:
		if len(scores) > 0 {
			// Find the highest-score line within this block
			bestIdx := 0
			bestScore := -1
			for i := range contentLines {
				globalIdx := startLine - 1 + i // 0-indexed into scores
				if globalIdx < len(scores) && scores[globalIdx] > bestScore {
					bestScore = scores[globalIdx]
					bestIdx = i
				}
			}
			// Center keepN lines around bestIdx
			start := bestIdx - keepN/2
			if start < 0 {
				start = 0
			}
			end := start + keepN
			if end > total {
				end = total
				start = end - keepN
				if start < 0 {
					start = 0
				}
			}
			return start, end
		}
		return 0, keepN

	case BlockTail:
		start := total - keepN
		if start < 0 {
			start = 0
		}
		return start, total

	default: // BlockHead and others
		return 0, keepN
	}
}

// minimalValidStructure returns the smallest valid structured output.
func minimalValidStructure(format OutputFormat, limit int) (string, error) {
	minimal := "{}\n"
	if len(minimal) <= limit {
		return minimal, nil
	}
	return "", fmt.Errorf("limit %d too small for structured output (minimum %d bytes)", limit, len(minimal))
}

// findLowestPriorityContentBlock returns the index of the content block
// that should be removed first. Returns -1 if no content blocks remain.
// Priority for keeping: match(3) > head(2) > tail(1).
// Among same priority, the larger block is removed first (more savings per step).
func findLowestPriorityContentBlock(blocks []Block) int {
	type candidate struct {
		index    int
		priority int
		size     int
	}
	var candidates []candidate
	for i, b := range blocks {
		if b.Type == BlockOmitted {
			continue
		}
		candidates = append(candidates, candidate{
			index:    i,
			priority: blockKeepPriority(b.Type),
			size:     len(b.Content),
		})
	}
	if len(candidates) == 0 {
		return -1
	}
	// Sort: lowest priority first; if tied, largest content first
	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].priority != candidates[b].priority {
			return candidates[a].priority < candidates[b].priority
		}
		return candidates[a].size > candidates[b].size
	})
	return candidates[0].index
}

// blockKeepPriority returns how important a block type is to keep.
// Higher value = more important to keep = removed last.
func blockKeepPriority(blockType string) int {
	switch blockType {
	case BlockMatch:
		return 3
	case BlockHead:
		return 2
	case BlockTail:
		return 1
	default:
		return 0
	}
}

// mergeAdjacentOmitted combines consecutive omitted blocks into one.
func mergeAdjacentOmitted(blocks []Block) []Block {
	if len(blocks) <= 1 {
		return blocks
	}
	merged := []Block{blocks[0]}
	for i := 1; i < len(blocks); i++ {
		last := &merged[len(merged)-1]
		if last.Type == BlockOmitted && blocks[i].Type == BlockOmitted {
			last.EndLine = blocks[i].EndLine
			last.OmittedCount += blocks[i].OmittedCount
		} else {
			merged = append(merged, blocks[i])
		}
	}
	return merged
}

// rebuildResult creates a new Result with updated blocks and recalculated counts.
func rebuildResult(original Result, blocks []Block) Result {
	kept := 0
	omitted := 0
	for _, b := range blocks {
		if b.Type == BlockOmitted {
			omitted += b.OmittedCount
		} else {
			kept += b.EndLine - b.StartLine + 1
		}
	}
	// Ensure omitted covers all non-kept lines
	if kept+omitted < original.TotalLines {
		omitted = original.TotalLines - kept
	}
	return Result{
		TotalLines:      original.TotalLines,
		KeptLines:       kept,
		OmittedLines:    omitted,
		PatternsMatched: original.PatternsMatched,
		Blocks:          blocks,
		EffectiveLimit:  original.EffectiveLimit,
	}
}

// renderStructured renders JSON or YAML without limit enforcement.
func renderStructured(result Result, format OutputFormat) (string, error) {
	switch format {
	case FormatJSON:
		return renderJSON(result)
	case FormatYAML:
		return renderYAML(result)
	default:
		return renderPlain(result), nil
	}
}

func renderPlain(result Result) string {
	if len(result.Blocks) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, b := range result.Blocks {
		if b.Type == BlockOmitted {
			sb.WriteString(fmt.Sprintf("... (%d lines omitted) ...\n", b.OmittedCount))
		} else {
			sb.WriteString(b.Content)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// yamlResult is the serialization structure for YAML/JSON output.
type yamlResult struct {
	Summary yamlSummary `json:"summary" yaml:"summary"`
	Blocks  []Block     `json:"blocks" yaml:"blocks"`
}

type yamlSummary struct {
	TotalLines      int `json:"total_lines" yaml:"total_lines"`
	KeptLines       int `json:"kept_lines" yaml:"kept_lines"`
	OmittedLines    int `json:"omitted_lines" yaml:"omitted_lines"`
	PatternsMatched int `json:"patterns_matched" yaml:"patterns_matched"`
}

func toYamlResult(r Result) yamlResult {
	return yamlResult{
		Summary: yamlSummary{
			TotalLines:      r.TotalLines,
			KeptLines:       r.KeptLines,
			OmittedLines:    r.OmittedLines,
			PatternsMatched: r.PatternsMatched,
		},
		Blocks: r.Blocks,
	}
}

func renderJSON(result Result) (string, error) {
	data, err := json.MarshalIndent(toYamlResult(result), "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(data) + "\n", nil
}

func renderYAML(result Result) (string, error) {
	data, err := yaml.Marshal(toYamlResult(result))
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}
	return string(data), nil
}
