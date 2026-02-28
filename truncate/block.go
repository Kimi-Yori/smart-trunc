package truncate

import (
	"strings"
)

// Block represents a contiguous section of the output.
type Block struct {
	Type         string `json:"type" yaml:"type"`
	StartLine    int    `json:"start_line" yaml:"start_line"`
	EndLine      int    `json:"end_line" yaml:"end_line"`
	Content      string `json:"content,omitempty" yaml:"content,omitempty"`
	OmittedCount int    `json:"omitted_count,omitempty" yaml:"omitted_count,omitempty"`
	Pattern      string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
}

// BlockType constants for block categorization.
const (
	BlockHead    = "head"
	BlockTail    = "tail"
	BlockMatch   = "match"
	BlockOmitted = "omitted"
)

// BuildKeepMap creates a boolean slice indicating which lines to keep.
// It marks head/tail lines and lines around high-score positions.
func BuildKeepMap(lineCount int, scores []int, head, tail, context int) []bool {
	// Clamp negative values to 0 for defensive safety
	if head < 0 {
		head = 0
	}
	if tail < 0 {
		tail = 0
	}
	if context < 0 {
		context = 0
	}

	keep := make([]bool, lineCount)

	// Mark head lines
	for i := 0; i < head && i < lineCount; i++ {
		keep[i] = true
	}

	// Mark tail lines
	for i := lineCount - tail; i < lineCount; i++ {
		if i >= 0 {
			keep[i] = true
		}
	}

	// Mark high-score lines and their context
	for i, score := range scores {
		if score > 0 {
			start := i - context
			if start < 0 {
				start = 0
			}
			end := i + context + 1
			if end > lineCount {
				end = lineCount
			}
			for j := start; j < end; j++ {
				keep[j] = true
			}
		}
	}

	return keep
}

// BuildBlocks groups consecutive kept/omitted lines into blocks.
func BuildBlocks(lines []string, keep []bool, scores []int, head, tail int) []Block {
	if len(lines) == 0 {
		return nil
	}

	var blocks []Block
	lineCount := len(lines)
	i := 0

	for i < lineCount {
		if keep[i] {
			// Collect consecutive kept lines
			start := i
			var content []string
			for i < lineCount && keep[i] {
				content = append(content, lines[i])
				i++
			}

			blockType := classifyBlock(start, i-1, lineCount, scores[start:i], head, tail)
			blocks = append(blocks, Block{
				Type:      blockType,
				StartLine: start + 1, // 1-indexed
				EndLine:   i,         // 1-indexed, inclusive
				Content:   strings.Join(content, "\n"),
			})
		} else {
			// Collect consecutive omitted lines
			start := i
			for i < lineCount && !keep[i] {
				i++
			}
			blocks = append(blocks, Block{
				Type:         BlockOmitted,
				StartLine:    start + 1,
				EndLine:      i,
				OmittedCount: i - start,
			})
		}
	}

	return blocks
}

// classifyBlock determines block type based on position and scores.
func classifyBlock(start, end, lineCount int, scores []int, head, tail int) string {
	// Check if any line in the block has a positive score
	hasMatch := false
	for _, s := range scores {
		if s > 0 {
			hasMatch = true
			break
		}
	}

	if hasMatch {
		return BlockMatch
	}

	// Head region
	if start < head {
		return BlockHead
	}

	// Tail region
	if end >= lineCount-tail {
		return BlockTail
	}

	// Context around a match (kept but no direct match)
	return BlockMatch
}
